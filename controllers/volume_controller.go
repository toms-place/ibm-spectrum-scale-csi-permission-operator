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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	permissionsv1alpha1 "bigdata.wu.ac.at/filepermissions/v1/api/v1alpha1"
)

type VolumeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

const (
	//TODO: make configurable
	StorageClassToFilter string = "ibm-spectrum-scale-csi-fileset"
	pvCSIDriverToFilter  string = "spectrumscale.csi.ibm.com"
)

//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
//+kubebuilder:rbac:groups="",resources=PersistentVolumes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumes/spec,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumes/status,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumeClaims,verbs=get;list
func (r *VolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	_ = log.FromContext(ctx)
	reqLogger := r.Log.WithValues("ReconcileVolumeController", req.NamespacedName)

	var fp permissionsv1alpha1.FilePermissions
	var fpSpec permissionsv1alpha1.FilePermissionsSpec
	var fps permissionsv1alpha1.FilePermissionsList
	var pvs corev1.PersistentVolumeList
	var pvc corev1.PersistentVolumeClaim

	if err := r.Get(ctx, req.NamespacedName, &pvc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if *pvc.Spec.StorageClassName != StorageClassToFilter {
		return ctrl.Result{}, nil
	}

	listOpts := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(".metadata.name", pvc.Spec.VolumeName),
	}

	if err := r.List(ctx, &pvs, listOpts); err != nil {
		reqLogger.Error(err, "unable to fetch pvcs")
		return ctrl.Result{}, err
	}

	if len(pvs.Items) > 0 && pvs.Items[0].Spec.CSI.Driver == pvCSIDriverToFilter {

		listOpts := &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(".spec.pvcrefuid", string(pvc.UID)),
		}

		if err := r.List(ctx, &fps, listOpts); err != nil {
			reqLogger.Error(err, "unable to fetch FPs")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		if len(fps.Items) == 0 {

			fp.Name = "fp-" + pvs.Items[0].Name
			fpSpec.PvRefUID = pvs.Items[0].UID
			fpSpec.PvcRefUID = pvc.UID
			fpSpec.PvcNamespace = req.Namespace
			fpSpec.PvcName = req.Name
			fp.Spec = fpSpec

			reqLogger.Info("CREATE FP", "fp.Spec.PvcName", pvc.Name)

			if err := r.Create(ctx, &fp); err != nil {
				reqLogger.Error(err, "unable to create", "FilePermissions", fp.Name)
				return ctrl.Result{}, err
			}

		}

		if !pvc.DeletionTimestamp.IsZero() {
			reqLogger.Info("DELETE FPs")

			for i := range fps.Items {
				if err := r.Delete(ctx, &fps.Items[i]); err != nil {
					reqLogger.Error(err, "unable to delete", "fp", fps.Items[i].Name)
					return ctrl.Result{}, err
				}
			}
		}

	}

	return ctrl.Result{}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *VolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&permissionsv1alpha1.FilePermissions{}).
		Watches(
			&source.Kind{Type: &corev1.PersistentVolumeClaim{}},
			handler.EnqueueRequestsFromMapFunc(r.PersistentVolumeClaimHandler),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

//+kubebuilder:rbac:groups="",resources=PersistentVolumesClaims,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumesClaims/status,verbs=get;list;watch
func (r *VolumeReconciler) PersistentVolumeClaimHandler(object client.Object) []reconcile.Request {
	requests := make([]reconcile.Request, 1)
	requests[0] = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      object.GetName(),
			Namespace: object.GetNamespace(),
		},
	}
	return requests
}
