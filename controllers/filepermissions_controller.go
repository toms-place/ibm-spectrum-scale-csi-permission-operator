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

/*

TODO:
- make 2 controllers accessing same state.
- watching claims and pvs
- creating permissions on claims
- executing permissions on pvs
- deleting permissions if not relevant
- see: https://book.kubebuilder.io/reference/watching-resources/externally-managed.html
- turn around logic
*/

package controllers

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/martian/filter"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client" // Required for Watching
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	// Required for Watching
	// Required for Watching
	permissionsv1alpha1 "bigdata.wu.ac.at/filepermissions/v1/api/v1alpha1"
)

const (
	pvRefField = ".spec.PvRef"
)

type PersistentVolumeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
//+kubebuilder:rbac:groups="",resources=PersistentVolumes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumes/spec,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumes/status,verbs=get;list;watch
func (r *PersistentVolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	_ = log.FromContext(ctx)
	reqLogger := r.Log.WithValues("PV:", req.NamespacedName)

	var pv corev1.PersistentVolume
	var FilePermissions permissionsv1alpha1.FilePermissions
	var cronJobs batchv1.JobList

	if err := r.Get(ctx, req.NamespacedName, &pv); err != nil {
		reqLogger.Error(err, "unable to fetch PersistentVolume")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if strings.Contains(pv.Spec.StorageClassName, "ibm-spectrum-scale-csi-fileset") {
		reqLogger.Info(pv.Spec.String())
	}

	if err := r.Get(ctx, req.NamespacedName, &FilePermissions); err != nil {
		//reqLogger.Error(err, "unable to fetch FilePermissions")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	//reqLogger.Info(FilePermissions.Spec.Owner)

	if err := r.List(ctx, &cronJobs); err != nil {
		reqLogger.Error(err, "unable to list cronJobs")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	isJobFinished := func(job *batchv1.Job) (bool, batchv1.JobConditionType) {
		for _, c := range job.Status.Conditions {
			if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
				return true, c.Type
			}
		}

		return false, ""
	}

	var activeJobs []*batchv1.Job
	var successfulJobs []*batchv1.Job
	var failedJobs []*batchv1.Job

	for i, job := range cronJobs.Items {
		_, finishedType := isJobFinished(&job)
		switch finishedType {
		case "": // ongoing
			activeJobs = append(activeJobs, &cronJobs.Items[i])
		case batchv1.JobFailed:
			failedJobs = append(failedJobs, &cronJobs.Items[i])
		case batchv1.JobComplete:
			successfulJobs = append(successfulJobs, &cronJobs.Items[i])
		}
	}

	reqLogger.Info(successfulJobs[len(successfulJobs)-1].Name)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PersistentVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&permissionsv1alpha1.FilePermissions{}).
		Owns(&batchv1.Job{}).
		Watches(
			&source.Kind{Type: &corev1.PersistentVolume{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForConfigMap),
			builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
				object.GetCreationTimestamp().Time
			}))
		).
		Complete(r)
}

/*

const (
	pvRefField = ".spec.PvRef"
)

type PersistentVolumeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
//+kubebuilder:rbac:groups="",resources=PersistentVolumes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumes/spec,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumes/status,verbs=get;list;watch
func (r *PersistentVolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	_ = log.FromContext(ctx)
	reqLogger := r.Log.WithValues("PV:", req.NamespacedName)

	var pv corev1.PersistentVolume
	var FilePermissions permissionsv1alpha1.FilePermissions
	var cronJobs batchv1.JobList

	if err := r.Get(ctx, req.NamespacedName, &pv); err != nil {
		reqLogger.Error(err, "unable to fetch PersistentVolume")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if strings.Contains(pv.Spec.StorageClassName, "ibm-spectrum-scale-csi-fileset") {
		reqLogger.Info(pv.Spec.String())
	}

	if err := r.Get(ctx, req.NamespacedName, &FilePermissions); err != nil {
		//reqLogger.Error(err, "unable to fetch FilePermissions")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	//reqLogger.Info(FilePermissions.Spec.Owner)

	if err := r.List(ctx, &cronJobs); err != nil {
		reqLogger.Error(err, "unable to list cronJobs")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	isJobFinished := func(job *batchv1.Job) (bool, batchv1.JobConditionType) {
		for _, c := range job.Status.Conditions {
			if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
				return true, c.Type
			}
		}

		return false, ""
	}

	var activeJobs []*batchv1.Job
	var successfulJobs []*batchv1.Job
	var failedJobs []*batchv1.Job

	for i, job := range cronJobs.Items {
		_, finishedType := isJobFinished(&job)
		switch finishedType {
		case "": // ongoing
			activeJobs = append(activeJobs, &cronJobs.Items[i])
		case batchv1.JobFailed:
			failedJobs = append(failedJobs, &cronJobs.Items[i])
		case batchv1.JobComplete:
			successfulJobs = append(successfulJobs, &cronJobs.Items[i])
		}
	}


	reqLogger.Info(successfulJobs[len(successfulJobs)-1].Name)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PersistentVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.PersistentVolume{}).
		Owns(&batchv1.Job{}).
		Owns(&permissionsv1alpha1.FilePermissions{}).
		Complete(r)
}

*/
