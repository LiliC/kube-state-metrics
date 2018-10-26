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

// TODO: rename collector
package collectors

import (
	"fmt"
	"strings"

	autoscaling "k8s.io/api/autoscaling/v2beta1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	extensions "k8s.io/api/extensions/v1beta1"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kube-state-metrics/pkg/metrics"
	metricsstore "k8s.io/kube-state-metrics/pkg/metrics_store"
	"k8s.io/kube-state-metrics/pkg/options"
	"k8s.io/kube-state-metrics/pkg/uclient"
)

// Builder helps to build collectors. It follows the builder pattern
// (https://en.wikipedia.org/wiki/Builder_pattern).
type Builder struct {
	kubeClient        clientset.Interface
	namespaces        options.NamespaceList
	opts              *options.Options
	ctx               context.Context
	enabledCollectors options.CollectorSet
}

// NewBuilder returns a new builder.
func NewBuilder(
	ctx context.Context,
	opts *options.Options,
) *Builder {
	return &Builder{
		opts: opts,
		ctx:  ctx,
	}
}

// WithEnabledCollectors sets the enabledCollectors property of a Builder.
func (b *Builder) WithEnabledCollectors(c options.CollectorSet) {
	b.enabledCollectors = c
}

// WithNamespaces sets the namespaces property of a Builder.
func (b *Builder) WithNamespaces(n options.NamespaceList) {
	b.namespaces = n
}

// WithKubeClient sets the kubeClient property of a Builder.
func (b *Builder) WithKubeClient(c clientset.Interface) {
	b.kubeClient = c
}

// Build initializes and registers all enabled collectors.
func (b *Builder) Build() []*Collector {

	collectors := []*Collector{}
	activeCollectorNames := []string{}

	for c := range b.enabledCollectors {
		constructor, ok := availableCollectors[c]
		if ok {
			collector := constructor(b)
			activeCollectorNames = append(activeCollectorNames, c)
			collectors = append(collectors, collector)
		}
		// TODO: What if not ok?
	}

	glog.Infof("Active collectors: %s", strings.Join(activeCollectorNames, ","))

	return collectors
}

var availableCollectors = map[string]func(f *Builder) *Collector{
	"configmaps": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "v1", "ConfigMap", generateConfigMapMetrics)
	},
	"cronjobs": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "batch/v1beta1", "CronJob", generateCronJobMetrics)
	},
	"deamonsets": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "extensions/v1beta1", "DeamonSet", generateDeamonSetMetrics)
	},
	"deployments": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "apps/v1beta1", "Deployment", generateDeploymentMetrics)
	},
	"endpoints": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "v1", "Endpoints", generateEndpointsMetrics)
	},
	"horizontalpodautoscalers": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "autoscaling/v2beta1", "HorizontalPodAutoscaler", generateHorizontalPodAutoscalerMetrics)
	},
	"jobs": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "batch/v1", "Job", generateJobMetrics)
	},
	"limitranges": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "v1", "LimitRange", generateLimitRangeMetrics)
	},
	"namespaces": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "v1", "Namespace", generateNamespaceMetrics)
	},
	"nodes": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "v1", "Node", generateNodeMetrics)
	},
	"persistentvolumeclaims": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "v1", "PersistentVolumeClaim", generatePersistentVolumeClaimMetrics)
	},
	"persistentvolumes": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "v1", "PersistentVolume", generatePersistentVolumeMetrics)
	},
	"poddisruptionbudgets": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "apps/v1beta1", "PodDisruptionBudget", generatePodDisruptionBudgetMetrics)
	},
	"pods": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "v1", "Pod", generatePodMetrics)
	},
	"replicasets": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "apps/v1beta1", "ReplicaSet", generateReplicaSetMetrics)
	},
	"replicationcontrollers": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "v1", "ReplicationController", generateReplicationControllerMetrics)
	},
	"resourcequotas": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "v1", "ResourceQuota", generateResourceQuotaMetrics)
	},
	"secrets": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "v1", "Secret", generateSecretMetrics)
	},
	"services": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "v1", "Service", generateServiceMetrics)
	},
	"statefulsets": func(b *Builder) *Collector {
		return BuildCollector(b.namespaces, "apps/v1beta1", "StatefulSet", generateStatefulSetMetrics)
	},
}

func BuildCollector(namespaces []string,
	api string,
	kind string,
	generateStore func(obj interface{}) []*metrics.Metric) *Collector {
	store := metricsstore.NewMetricsStore(generateStore)
	reflectorPerNs(context.TODO(), &unstructured.Unstructured{}, store, namespaces, api, kind)
	return newCollector(store)
}

func reflectorPerNs(
	ctx context.Context,
	expectedType interface{},
	store cache.Store,
	namespaces []string,
	api string,
	kind string,
) {
	for _, ns := range namespaces {
		cfg, err := clientcmd.BuildConfigFromFlags("", "")
		if err != nil {
			fmt.Println(err)
			return
		}
		uc := uclient.NewForConfig(cfg)
		dclient, err := uc.ClientFor(api, kind, ns)
		if err != nil {
			fmt.Println(err)
			return
		}
		lw := listWatchFunc(dclient, ns)
		reflector := cache.NewReflector(&lw, expectedType, store, 0)
		go reflector.Run(ctx.Done())
	}
}

func listWatchFunc(dynamicInterface dynamic.NamespaceableResourceInterface, namespace string) cache.ListWatch {
	return cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return dynamicInterface.Namespace(namespace).List(opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return dynamicInterface.Namespace(namespace).Watch(opts)
		},
	}
}
