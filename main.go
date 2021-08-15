/*
Copyright 2021.

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
	"context"
	"flag"
	"fmt"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/utils/pointer"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	viewv1 "github.com/satoru-takeuchi/markdown-viewer/api/v1"
	"github.com/satoru-takeuchi/markdown-viewer/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(viewv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "0c4a5fac.satoru-takeuchi.github.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.MarkDownViewReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MarkDownView")
		os.Exit(1)
	}
	if err = (&viewv1.MarkDownView{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "MarkDownView")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func get(ctx context.Context, cli client.Client) error {
	var deployment appsv1.Deployment
	err := cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: "sample"}, &deployment)
	if err != nil {
		return err
	}
	fmt.Printf("Got Deployment: %#v\n", deployment)
	return nil
}

func list(ctx context.Context, cli client.Client) error {
	var pods corev1.PodList
	err := cli.List(ctx, &pods, &client.ListOptions{
		Namespace: "default",
		LabelSelector: labels.SelectorFromSet(
			map[string]string{"app": "sample"}),
	})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		fmt.Println(pod.Name)
	}
	return nil
}

func pagination(ctx context.Context, cli client.Client) error {
	token := ""
	for i := 0; ; i++ {
		var pods corev1.PodList
		err := cli.List(ctx, &pods, &client.ListOptions{
			Limit:    3,
			Continue: token,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Page %d:\n", i)
		for _, pod := range pods.Items {
			fmt.Println(pod.Name)
		}

		token := pods.ListMeta.Continue
		if len(token) == 0 {
			return nil
		}
	}
}

func create(ctx context.Context, cli client.Client) error {
	dep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "nginx"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "nginx"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
	err := cli.Create(ctx, &dep)
	if err != nil {
		return err
	}
	return nil
}

func createOrUpdate(ctx context.Context, cli client.Client) error {
	dep := &appsv1.Deployment{}
	dep.SetNamespace("default")
	dep.SetName("sample")

	op, err := ctrl.CreateOrUpdate(ctx, cli, dep, func() error {
		dep.Spec.Replicas = pointer.Int32Ptr(1)
		dep.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "nginx"},
		}
		dep.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "nginx"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:latest",
					},
				},
			},
		}
		return nil
	})
	if err != nil {
		return err
	}
	if op != controllerutil.OperationResultNone {
		fmt.Printf("Deployment %s\n", op)
	}
	return nil
}

func patchMerge(ctx context.Context, cli client.Client) error {
	var dep appsv1.Deployment
	err := cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: "sample"}, &dep)
	if err != nil {
		return err
	}

	newDep := dep.DeepCopy()
	newDep.Spec.Replicas = pointer.Int32Ptr(3)
	patch := client.MergeFrom(&dep)
	err = cli.Patch(ctx, newDep, patch)
	return err
}

func patchApply(ctx context.Context, cli client.CLient) error {
	patch := &unstructured.Unstructured{}
	patch.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	})
	patch.SetNamespace("default")
	patch.Setname("sample2")
	patch.UnstructuredContent()["spec"] = map[string]interface{}{
		"replicas": 2,
		"selector": map[string]interface{}{
			"matchLabels": map[string]string{
				"app": "nginx",
			},
		},
		"template": map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]string{
					"app": "nginx",
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "nginx",
						"image": "nginx:latest",
					},
				},
			},
		},
	}

	err := cli.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "client-sample",
		Force:        pointer.Bool(true),
	})
	return err
}

func patchApplyConfig(ctx context.Context, cli clientClient) error {
	dep := appsv1apply.Deployment("sample3", "default").
		WithSpec(appsv1apply.DeploymentSpec().
			WithReplicas(3).
			WithSelector(metav1apply.LabelSelector().WithMatchLabels(map[string]string{"app": "nginx"})).
			WithTemplate(corev1apply.PodTemplateSPec().
				WithLabels(map[string]string{"app": "nginx"}).
				WithSpec(corev1apply.PodSpec().
					WithContainers(corev1apply.Container().
						WithName("nginx").
						WithImage("nginx:latest"),
					),
				),
			),
		)

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dep)
	if err != nil {
		return err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}

	var current appsv1.Deployment
	err = cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: "sample3"}, &current)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	currApplyConfig, err := appsv1apply.ExtractDeployment(&current, "client-sample")
	if err != nil {
		return nil
	}

	if equality.Semantic.DeepEqual(dep, curApplyConfig) {
		return nil
	}

	err = cli.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "client-sample",
		Force:        pointer.Bool(true),
	})
	return err
}

func updateStatus(ctx context.Context, cli client.Client) error {
	var dep appsv1.Deployment
	err := cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: "sample"}, &dep)
	if err != nil {
		return err
	}

	dep.Status.AvailableReplicas = 3
	err = cli.Status().Update(ctx, &dep)
	return err
}

func deleteWithPreConditions(ctx context.Context, cli client.Client) error {
	var deploy appsv1.Deployment
	err := cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: "sample"}, &deploy)
	if err != nil {
		return err
	}
	uid := deploy.GetUID()
	resourceVersion := deploy.GetResourceVersion()
	cond := metav1.Preconditions{
		UID:             &uid,
		ResourceVersion: &resourceVersion,
	}
	err = cli.Delete(ctx, &deploy, &client.DeleteOptions{
		Preconditions: &cond,
	})
	return err
}

func deleteAllOfDeployment(ctx context.Context, cli client.Client) error {
	err := cli.DeleteAllOf(ctx, &appsv1.Deployment{}, client.InNamespace("default"))
	return err
}
