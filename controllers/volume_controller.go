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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	permissionsv1alpha1 "bigdata.wu.ac.at/filepermissions/v1/api/v1alpha1"
)

type VolumeControllerReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	Log              logr.Logger
	PvCSIDriverField string
	PvStatusField    string
}

func isTrue() *bool {
	b := true
	return &b
}

//var pvSpec corev1.PersistentVolumeSpec

const (
	pvCSIDriverToFilter = "spectrumscale.csi.ibm.com"
	//TODO: use storageclass and make configurable
	StorageClassToFilter = ""
)

//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=PersistentVolumes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumes/spec,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumes/status,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumeClaims,verbs=get;list
func (r *VolumeControllerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	_ = log.FromContext(ctx)
	reqLogger := r.Log.WithValues("VolumeController:", req.NamespacedName)

	var pv corev1.PersistentVolume
	var pvc corev1.PersistentVolumeClaim
	var fp permissionsv1alpha1.FilePermissions
	var fpSpec permissionsv1alpha1.FilePermissionsSpec

	if err := r.Get(ctx, req.NamespacedName, &pvc); err != nil {
		reqLogger.Error(err, "unable to fetch PersistentVolumeClaim")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.Get(ctx, types.NamespacedName{Name: pvc.Spec.VolumeName}, &pv); err != nil {
		reqLogger.Error(err, "unable to fetch PersistentVolume")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	fpName := pv.Name

	if err := r.Get(ctx, types.NamespacedName{Name: fpName}, &fp); err != nil {
		client.IgnoreNotFound(err)
	}

	if fp.Name != fpName {

		fpSpec.PvRefUID = pv.UID
		fpSpec.PvcRefUID = pvc.UID
		fpSpec.PvcName = pvc.Name
		fpSpec.PvcNamespace = pvc.Namespace
		fp.Name = fpName
		fp.Spec = fpSpec

		if err := r.Create(ctx, &fp); err != nil {
			reqLogger.Error(err, "unable to create", "FilePermissions", fp)
			return ctrl.Result{}, err
		}

	}

	return ctrl.Result{}, nil
}

//+kubebuilder:rbac:groups="",resources=PersistentVolumes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumes/spec,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumes/status,verbs=get;list;watch
func (r *VolumeControllerReconciler) PersistentVolumeClaimFilter(object client.Object) bool {
	reqLogger := r.Log.WithValues("Init_FILTER:", object.GetName())

	filterFlag := false
	pvs := &corev1.PersistentVolumeList{}

	/*
		not working with cache..
		see ISSUE: https://github.com/kubernetes-sigs/controller-runtime/issues/612#issuecomment-914911022
		Codeline: https://github.com/kubernetes-sigs/controller-runtime/blob/v0.10.2/pkg/cache/internal/cache_reader.go#L117

		listOpts := &client.ListOptions{
			FieldSelector: fields.AndSelectors(
				fields.OneTermEqualSelector(r.PvCSIDriverField, pvCSIDriverToFilter),
				fields.OneTermEqualSelector(r.PvStatusField, string(corev1.VolumeBound)),
			),
		}
	*/

	pvlistOpts := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(r.PvCSIDriverField, pvCSIDriverToFilter),
	}

	if err := r.List(context.Background(), pvs, pvlistOpts); err != nil {
		reqLogger.Error(err, "unable to fetch PersistentVolumes")
	}

	for i := range pvs.Items {

		if pvs.Items[i].Spec.ClaimRef.UID == object.GetUID() && pvs.Items[i].Status.Phase == corev1.VolumeBound {
			reqLogger.Info(pvs.Items[i].Spec.CSI.Driver)
			reqLogger.Info(string(pvs.Items[i].Status.Phase))
			filterFlag = true
		}

	}

	return filterFlag

}

// SetupWithManager sets up the controller with the Manager.
func (r *VolumeControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}).
		WithEventFilter(predicate.NewPredicateFuncs(r.PersistentVolumeClaimFilter)).
		Complete(r)
}
