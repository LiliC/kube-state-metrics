/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kube-state-metrics/pkg/options"
	"k8s.io/kube-state-metrics/pkg/uclient"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	kcollectors "k8s.io/kube-state-metrics/pkg/collectors"
)

func BenchmarkKubeStateMetrics(t *testing.B) {
	fixtureMultiplier := 1000
	requestCount := 100

	t.Logf(
		"starting kube-state-metrics benchmark with fixtureMultiplier %v and requestCount %v",
		fixtureMultiplier,
		requestCount,
	)

	//kubeClient := fake.NewSimpleClientset()
	cfg, err := clientcmd.BuildConfigFromFlags("", "/Users/lili/.kube/config")
	if err != nil {
		t.Errorf("error injecting resources: %v", err)
	}
	uc := uclient.NewForConfig(cfg)
	/*
		if err := injectFixtures(kubeClient, fixtureMultiplier); err != nil {
			t.Errorf("error injecting resources: %v", err)
		}
	*/
	opts := options.NewOptions()

	builder := kcollectors.NewBuilder(context.TODO(), opts)
	builder.WithEnabledCollectors(options.DefaultCollectors)
	builder.WithDynamicClient(uc)
	builder.WithNamespaces(options.DefaultNamespaces)

	collectors := builder.Build()

	handler := MetricHandler{collectors, false}

	req := httptest.NewRequest("GET", "http://localhost:8080/metrics", nil)

	// Wait for informers to sync
	time.Sleep(time.Second)

	var w *httptest.ResponseRecorder
	for i := 0; i < requestCount; i++ {
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)

	t.Errorf("error injecting resources: %s", string(body))
	fmt.Println(resp.StatusCode)
	fmt.Println(resp.Header.Get("Content-Type"))
	fmt.Println(string(body))
}

func injectFixtures(client *fake.Clientset, multiplier int) error {
	creators := []func(*fake.Clientset, int) error{
		configMap,
		//pod,
	}

	for _, c := range creators {
		for i := 0; i < multiplier; i++ {
			err := c(client, i)

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func configMap(client *fake.Clientset, index int) error {
	i := strconv.Itoa(index)

	configMap := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "configmap" + i,
			ResourceVersion: "123456",
		},
	}
	_, err := client.CoreV1().ConfigMaps(metav1.NamespaceDefault).Create(&configMap)
	return err
}

func pod(client *fake.Clientset, index int) error {
	i := strconv.Itoa(index)

	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod" + i,
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				v1.ContainerStatus{
					Name:        "container1",
					Image:       "k8s.gcr.io/hyperkube1",
					ImageID:     "docker://sha256:aaa",
					ContainerID: "docker://ab123",
				},
			},
		},
	}

	_, err := client.CoreV1().Pods(metav1.NamespaceDefault).Create(&pod)
	return err
}

type MetricHandler struct {
	c                  []*kcollectors.Collector
	enableGZIPEncoding bool
}

func (m *MetricHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resHeader := w.Header()
	var writer io.Writer = w

	resHeader.Set("Content-Type", `text/plain; version=`+"0.0.4")

	if m.enableGZIPEncoding {
		// Gzip response if requested. Taken from
		// github.com/prometheus/client_golang/prometheus/promhttp.decorateWriter.
		reqHeader := r.Header.Get("Accept-Encoding")
		parts := strings.Split(reqHeader, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "gzip" || strings.HasPrefix(part, "gzip;") {
				writer = gzip.NewWriter(writer)
				resHeader.Set("Content-Encoding", "gzip")
			}
		}
	}

	for _, c := range m.c {
		for _, m := range c.Collect() {
			_, err := fmt.Fprint(writer, *m)
			if err != nil {
				// TODO: Handle panic
				panic(err)
			}
		}
	}

	// In case we gziped the response, we have to close the writer.
	if closer, ok := writer.(io.Closer); ok {
		closer.Close()
	}
}
