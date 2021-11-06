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
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	permissionsv1alpha1 "bigdata.wu.ac.at/filepermissions/v1/api/v1alpha1"
	"bigdata.wu.ac.at/filepermissions/v1/controllers"
	corev1 "k8s.io/api/core/v1"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const (
	PvCSIDriverField    = ".spec.csi.driver"
	PvStatusField       = ".status.phase"
	PvcClaimRefUIDField = ".spec.claimRef.uid"
	PodOwnerKey         = ".metadata.labels.job-name"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(permissionsv1alpha1.AddToScheme(scheme))
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
		LeaderElectionID:       "232a9baf.bigdata.wu.ac.at",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.PersistentVolume{}, PvCSIDriverField, func(rawObj client.Object) []string {
		// Extract the CSIDriverField name from the PersistentVolume Spec, if one is provided
		PersistentVolume := rawObj.(*corev1.PersistentVolume)
		if PersistentVolume.Spec.CSI.Driver == "" {
			return nil
		}
		return []string{PersistentVolume.Spec.CSI.Driver}
	}); err != nil {
		setupLog.Error(err, "unable to index")
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.PersistentVolume{}, PvStatusField, func(rawObj client.Object) []string {
		// Extract the CSIDriverField name from the PersistentVolume Spec, if one is provided
		PersistentVolume := rawObj.(*corev1.PersistentVolume)
		if PersistentVolume.Status.Phase == "" {
			return nil
		}
		return []string{string(PersistentVolume.Status.Phase)}
	}); err != nil {
		setupLog.Error(err, "unable to index")
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.PersistentVolume{}, PvcClaimRefUIDField, func(rawObj client.Object) []string {
		// Extract the CSIDriverField name from the PersistentVolume Spec, if one is provided
		PersistentVolume := rawObj.(*corev1.PersistentVolume)
		if PersistentVolume.Spec.CSI.Driver == "" {
			return nil
		}
		return []string{PersistentVolume.Spec.CSI.Driver}
	}); err != nil {
		setupLog.Error(err, "unable to index")
	}

	if err = (&controllers.FilePermissionsReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		Log:              mgr.GetLogger(),
		PvCSIDriverField: PvCSIDriverField,
		PvStatusField:    PvStatusField,
		PodOwnerKey:      PodOwnerKey,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "FilePermissions")
		os.Exit(1)
	}

	if err = (&controllers.VolumeControllerReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		Log:              mgr.GetLogger(),
		PvCSIDriverField: PvCSIDriverField,
		PvStatusField:    PvStatusField,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VolumeController")
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
