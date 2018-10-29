/*
Copyright 2018 The Kubernetes Authors All rights reserved.

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

package collectors

import (
	"k8s.io/kube-state-metrics/pkg/metrics"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	descConfigMapLabelsDefaultLabels = []string{"namespace", "configmap"}

	descConfigMapInfo = newMetricFamilyDef(
		"kube_configmap_info",
		"Information about configmap.",
		descConfigMapLabelsDefaultLabels,
		nil,
	)

	descConfigMapCreated = newMetricFamilyDef(
		"kube_configmap_created",
		"Unix creation timestamp",
		descConfigMapLabelsDefaultLabels,
		nil,
	)

	descConfigMapMetadataResourceVersion = newMetricFamilyDef(
		"kube_configmap_metadata_resource_version",
		"Resource version representing a specific version of the configmap.",
		append(descConfigMapLabelsDefaultLabels, "resource_version"),
		nil,
	)
)

func generateConfigMapMetrics(obj interface{}) []*metrics.Metric {
	ms := []*metrics.Metric{}

	// TODO: Refactor
	mPointer := obj.(*unstructured.Unstructured)
	m := *mPointer

	addConstMetric := func(desc *metricFamilyDef, v float64, lv ...string) {
		lv = append([]string{m.GetNamespace(), m.GetName()}, lv...)

		m, err := metrics.NewMetric(desc.Name, desc.LabelKeys, lv, v)
		if err != nil {
			panic(err)
		}

		ms = append(ms, m)
	}
	addGauge := func(desc *metricFamilyDef, v float64, lv ...string) {
		addConstMetric(desc, v, lv...)
	}
	addGauge(descConfigMapInfo, 1)

	//if m.GetCreationTimestamp() {
	addGauge(descConfigMapCreated, float64(m.GetCreationTimestamp().Unix()))
	//}

	addGauge(descConfigMapMetadataResourceVersion, 1, string(m.GetResourceVersion()))

	return ms
}
