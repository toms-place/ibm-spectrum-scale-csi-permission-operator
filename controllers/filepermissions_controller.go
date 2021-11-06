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
	Scheme           *runtime.Scheme
	Log              logr.Logger
	PvCSIDriverField string
	PvStatusField    string
	PodOwnerKey      string
}

//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=permissions.bigdata.wu.ac.at,resources=filepermissions/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
//+kubebuilder:rbac:groups="",resources=PersistentVolumes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumes/spec,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumes/status,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=PersistentVolumeClaims,verbs=get;list
func (r *FilePermissionsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	_ = log.FromContext(ctx)
	reqLogger := r.Log.WithValues("FP:", req.NamespacedName)

	var fp permissionsv1alpha1.FilePermissions
	var job batchv1.Job
	var crb rbacv1.ClusterRoleBinding
	var svcAcc corev1.ServiceAccount
	var podSpec corev1.PodTemplateSpec
	var volume corev1.Volume
	var volumes []corev1.Volume
	var container corev1.Container
	var containers []corev1.Container
	var volumemount corev1.VolumeMount
	var volumemounts []corev1.VolumeMount
	var toleration corev1.Toleration
	var tolerations []corev1.Toleration

	if err := r.Get(ctx, req.NamespacedName, &fp); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("no FilePermissions")
		} else {
			reqLogger.Info("unable to fetch FilePermissions")
			return ctrl.Result{}, err
		}
	}

	jobName := "test-job-" + fp.Name
	crbName := "test-crb-" + fp.Name
	svcAccName := "test-serviceaccount-" + fp.Name
	volumeName := "test-volume-" + fp.Name
	//permissionFinalizerName := "permissions.bigdata.wu.ac.at/finalizer"

	/*
		if !containsString(fp.GetFinalizers(), permissionFinalizerName) {
			controllerutil.AddFinalizer(&fp, permissionFinalizerName)
			if err := r.Update(ctx, &fp); err != nil {
				return ctrl.Result{}, err
			}
		}
	*/

	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: fp.Spec.PvcNamespace}, &job)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating Job", "NS:", req.Namespace)

		svcAcc.Name = svcAccName
		svcAcc.Namespace = fp.Spec.PvcNamespace

		if err := r.Create(ctx, &svcAcc); err != nil {
			reqLogger.Error(err, "unable to create", "svcAcc", svcAcc)
			return ctrl.Result{}, err
		}

		/*
			if !containsString(svcAcc.GetFinalizers(), permissionFinalizerName) {
				controllerutil.AddFinalizer(&svcAcc, permissionFinalizerName)
				if err := r.Update(ctx, &svcAcc); err != nil {
					return ctrl.Result{}, err
				}
			}
		*/

		crb.Name = crbName
		crb.RoleRef.Kind = "ClusterRole"
		crb.RoleRef.Name = "system-unrestricted-psp-role"
		crb.RoleRef.APIGroup = "rbac.authorization.k8s.io"

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
			if !containsString(crb.GetFinalizers(), permissionFinalizerName) {
				controllerutil.AddFinalizer(&crb, permissionFinalizerName)
				if err := r.Update(ctx, &crb); err != nil {
					return ctrl.Result{}, err
				}
			}
		*/

		/*
			//Ephemeral storage not supported
			// MountVolume.NewMounter initialization failed for volume "test-volume" : volume mode "Ephemeral" not supported by driver spectrumscale.csi.ibm.com (only supports ["Persistent"])

			volume.CSI = &corev1.CSIVolumeSource{
				Driver:           pv.Spec.CSI.Driver,
				VolumeAttributes: pv.Spec.CSI.VolumeAttributes,
			}
		*/

		volume.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{ClaimName: fp.Spec.PvcName}
		volume.Name = volumeName
		volumes = append(volumes, volume)

		volumemount.Name = volumeName
		volumemount.MountPath = "/mnt/dirtochange"

		volumemounts = append(volumemounts, volumemount)

		container.Image = "busybox"
		container.Name = "test-container"
		container.VolumeMounts = volumemounts

		var Command []string
		Command = append(Command, "chmod", "777", "/mnt/dirtochange/")
		container.Command = Command

		containers = append(containers, container)

		toleration.Key = "storage.provider"
		toleration.Effect = "NoSchedule"
		toleration.Value = "spectrum-scale"
		tolerations = append(tolerations, toleration)

		var labels map[string]string
		labels = make(map[string]string)
		labels["job"] = "permissions.bigdata.wu.ac.at"

		podSpec.Spec.Volumes = volumes
		podSpec.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
		podSpec.Spec.Containers = containers
		podSpec.Spec.Tolerations = tolerations
		podSpec.Spec.ServiceAccountName = svcAccName
		podSpec.ObjectMeta.OwnerReferences = job.OwnerReferences
		nonRoot := false
		securityContext := &corev1.PodSecurityContext{
			RunAsNonRoot: &nonRoot,
		}
		podSpec.Spec.SecurityContext = securityContext

		// Set the information you care about
		job.Spec.Template = podSpec
		job.Namespace = fp.Spec.PvcNamespace
		job.Name = jobName
		job.Labels = labels

		if err := r.Create(ctx, &job); err != nil {
			reqLogger.Error(err, "unable to create", "job", job)
			return ctrl.Result{}, err
		}

	} else if err == nil {
		if job.Status.Succeeded == 1 {

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

			/*
				if err := r.Delete(ctx, &pod); err != nil {
					reqLogger.Error(err, "unable to delete", "pod", podName)
					return ctrl.Result{}, err
				}
			*/

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

			/*
				if err := r.Delete(ctx, &fp); err != nil {
					reqLogger.Error(err, "unable to delete")
					return ctrl.Result{}, err
				}
			*/

			reqLogger.Info("all permissions set")
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{Requeue: true}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FilePermissionsReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, r.PodOwnerKey, func(rawObj client.Object) []string {
		// grab the pod object, extract the owner...
		pod := rawObj.(*corev1.Pod)
		owner := metav1.GetControllerOf(pod)
		if owner == nil {
			return nil
		}
		// ...make sure it's a Job...
		if owner.Kind != "Job" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&permissionsv1alpha1.FilePermissions{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Complete(r)
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
