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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	permissionsv1alpha1 "bigdata.wu.ac.at/filepermissions/v1/api/v1alpha1"
)

type FilePermissionsReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Log         logr.Logger
	PodOwnerKey string
}

// +kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterrolebindings,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="policy",resources=podsecuritypolicies,verbs=use

func (r *FilePermissionsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	_ = log.FromContext(ctx)
	reqLogger := r.Log.WithValues("ReconcileFilePermissionController", req.NamespacedName)

	var fp permissionsv1alpha1.FilePermissions
	var job batchv1.Job
	var crb rbacv1.ClusterRoleBinding
	var svcAcc corev1.ServiceAccount

	if err := r.Get(ctx, req.NamespacedName, &fp); err != nil {
		client.IgnoreNotFound(err)
	} else if fp.Spec.PermissionsSet == false {

		jobName := "fp-job-" + fp.Name
		crbName := "fp-crb-" + fp.Name
		svcAccName := "fp-serviceaccount-" + fp.Name
		volumeName := "fp-volume-" + fp.Name

		svcAcc.Name = svcAccName
		svcAcc.Namespace = fp.Spec.PvcNamespace
		crb.Name = crbName
		crb.RoleRef.Kind = "ClusterRole"
		crb.RoleRef.Name = "system-unrestricted-psp-role"
		crb.RoleRef.APIGroup = "rbac.authorization.k8s.io"

		err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: fp.Spec.PvcNamespace}, &job)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Creating Job")

			if err := r.Create(ctx, &svcAcc); err != nil {
				reqLogger.Error(err, "unable to create", "svcAcc", svcAcc)
				return ctrl.Result{}, err
			}

			sjs := []rbacv1.Subject{}
			sj := rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      svcAcc.Name,
				Namespace: fp.Spec.PvcNamespace,
			}

			sjs = append(sjs, sj)

			crb.Subjects = sjs

			if err := r.Create(ctx, &crb); err != nil {
				reqLogger.Error(err, "unable to create", "crb", crb)
				return ctrl.Result{}, err
			}

			/*
				//Ephemeral storage not supported
				// MountVolume.NewMounter initialization failed for volume "test-volume" : volume mode "Ephemeral" not supported by driver spectrumscale.csi.ibm.com (only supports ["Persistent"])

				volume.CSI = &corev1.CSIVolumeSource{
					Driver:           pv.Spec.CSI.Driver,
					VolumeAttributes: pv.Spec.CSI.VolumeAttributes,
				}
			*/

			nonRoot := false
			job := batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jobName,
					Namespace: fp.Spec.PvcNamespace,
					Labels:    map[string]string{"job": "permissions.bigdata.wu.ac.at"},
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							OwnerReferences: job.OwnerReferences,
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{{
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: fp.Spec.PvcName,
									}},
								Name: volumeName,
							}},
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{{
								Image: "busybox",
								Name:  "change-filepermissions",
								VolumeMounts: []corev1.VolumeMount{{
									Name:      volumeName,
									MountPath: "/mnt/dirtochange",
								}},
								Command: []string{"chmod", "777", "/mnt/dirtochange/"},
							}},
							ServiceAccountName: svcAccName,
							Tolerations: []corev1.Toleration{{
								Key:    "storage.provider",
								Effect: "NoSchedule",
								Value:  "spectrum-scale",
							}},
							SecurityContext: &corev1.PodSecurityContext{
								RunAsNonRoot: &nonRoot,
							},
						},
					},
				},
			}

			if err := r.Create(ctx, &job); err != nil {
				reqLogger.Error(err, "unable to create", "job", job.Name)
				return ctrl.Result{}, err
			}

		} else if job.Status.Succeeded == 1 {

			fp.Spec.PermissionsSet = true

			if err := r.Update(ctx, &fp); err != nil {
				reqLogger.Error(err, "unable to update", "fp", fp.Name)
			}

			if err := r.Delete(ctx, &job); err != nil {
				reqLogger.Error(err, "unable to delete", "job", jobName)
				return ctrl.Result{}, err
			}

			var pods corev1.PodList
			if err := r.List(ctx, &pods, client.InNamespace(fp.Spec.PvcNamespace), client.MatchingFields{r.PodOwnerKey: jobName}); err != nil {
				reqLogger.Error(err, "unable to list child Jobs")
				return ctrl.Result{}, err
			}

			for i := range pods.Items {
				if err := r.Delete(ctx, &pods.Items[i]); err != nil {
					reqLogger.Error(err, "unable to delete", "pod", pods.Items[i].Name)
					return ctrl.Result{}, err
				}
			}

			if err := r.Get(ctx, types.NamespacedName{Name: svcAccName, Namespace: fp.Spec.PvcNamespace}, &svcAcc); err != nil {
				reqLogger.Error(err, "unable to get", "svcAcc", svcAccName)
				return ctrl.Result{}, err
			}
			if err := r.Delete(ctx, &svcAcc); err != nil {
				reqLogger.Error(err, "unable to delete", "svcAcc", svcAccName)
				return ctrl.Result{}, err
			}

			if err := r.Get(ctx, types.NamespacedName{Name: crbName, Namespace: fp.Spec.PvcNamespace}, &crb); err != nil {
				reqLogger.Error(err, "unable to get", "crb", crbName)
				return ctrl.Result{}, err
			}
			if err := r.Delete(ctx, &crb); err != nil {
				reqLogger.Error(err, "unable to delete", "crb", crbName)
				return ctrl.Result{}, err
			}

			reqLogger.Info("all permissions set")
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{Requeue: true}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FilePermissionsReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&permissionsv1alpha1.FilePermissions{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Complete(r)
}
