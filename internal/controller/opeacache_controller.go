/*
Copyright 2025.

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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	opeaoperatorv1alpha1 "github.com/opea-project/GenAIInfra/opea-operator/api/v1alpha1"
)

const (
	downloadJobImage = "ghcr.io/airren/opea-model-downloader:latest"
	downloadJobName  = "%s-downloader"

	OpeaCacheFinalizer = "opeacache.finalizers.opea.io"
)

// OpeaCacheReconciler reconciles a OpeaCache object
type OpeaCacheReconciler struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
	log      logr.Logger
}

func NewOpeaCacheReconciler(client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder, log logr.Logger) *OpeaCacheReconciler {
	return &OpeaCacheReconciler{
		Client:   client,
		scheme:   scheme,
		recorder: recorder,
		log:      log,
	}
}

// +kubebuilder:rbac:groups=opea-operator.opea.io,resources=opeacaches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=opea-operator.opea.io,resources=opeacaches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=opea-operator.opea.io,resources=opeacaches/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=create;delete;get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpeaCache object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *OpeaCacheReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	r.log = log.FromContext(ctx)
	var err error
	r.log.Info("Reconciling OpeaCache:", "name", req.NamespacedName)

	// Fetch the OpeaCache instance
	opeaCache := &opeaoperatorv1alpha1.OpeaCache{}
	if err := r.Get(ctx, req.NamespacedName, opeaCache); err != nil {
		r.log.Error(err, "unable to fetch OpeaCache", "namespace", req.NamespacedName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	previousPhase := opeaCache.Status.Phase

	defer func() {
		if err != nil {
			r.recorder.Eventf(opeaCache, v1.EventTypeWarning, "ReconcileFailed",
				"OpeaCache %s reconciled failed: %%s", opeaCache.Name, err.Error())
		} else if previousPhase != opeaCache.Status.Phase {
			r.recorder.Eventf(opeaCache, v1.EventTypeNormal, "ReconcileSuccess",
				"OpeaCache %s reconciled success: %s -> %s", opeaCache.Name, previousPhase, opeaCache.Status.Phase)
		}
	}()

	// Check if the OpeaCache instance is marked for deletion
	if opeaCache.GetDeletionTimestamp().IsZero() {
		// The object is not being deleted
		if !controllerutil.ContainsFinalizer(opeaCache, OpeaCacheFinalizer) {
			controllerutil.AddFinalizer(opeaCache, OpeaCacheFinalizer)
			if err := r.Update(ctx, opeaCache); err != nil {
				r.log.Error(err, "unable to update OpeaCache with finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(opeaCache, OpeaCacheFinalizer) {
			// delete the cache
			// delete the pvc
			// remove the finalizer
			controllerutil.RemoveFinalizer(opeaCache, OpeaCacheFinalizer)
			if err := r.Update(ctx, opeaCache); err != nil {
				r.log.Error(err, "unable to update OpeaCache without finalizer")
				return ctrl.Result{}, err
			}
		}
	}
	return r.reconcileOpeaCache(ctx, opeaCache)
}

func (r *OpeaCacheReconciler) reconcileOpeaCache(ctx context.Context, opeaCache *opeaoperatorv1alpha1.OpeaCache) (res ctrl.Result, err error) {
	// check if the cache is ready

	// check if the pvc is ready
	err = r.reconcilePVC(ctx, opeaCache)
	if err != nil {
		r.log.Error(err, "unable to reconcile pvc")
		return ctrl.Result{}, err
	}

	// check if the download job is ready
	res, err = r.reconcileJob(ctx, opeaCache)
	if err != nil {
		r.log.Error(err, "unable to reconcile job")
		return ctrl.Result{}, err
	}

	return res, nil
}

func (r *OpeaCacheReconciler) reconcilePVC(ctx context.Context, opeaCache *opeaoperatorv1alpha1.OpeaCache) error {
	pvc := &v1.PersistentVolumeClaim{}
	pvc.Name = opeaCache.GetPVCName()
	pvc.Namespace = opeaCache.Namespace
	if err := r.Get(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil && client.IgnoreNotFound(err) != nil {
		r.log.Error(err, "unable to get pvc", "namespace", opeaCache.Namespace, "name", opeaCache.Name)
		return err
	} else if err != nil {
		pvc := r.constructPVC(opeaCache)
		if err := ctrl.SetControllerReference(opeaCache, pvc, r.scheme); err != nil {
			r.log.Error(err, "unable to set controller reference for pvc")
			return err
		}
		if err := r.Create(ctx, pvc); err != nil {
			r.log.Error(err, "unable to create pvc")
			return err
		}
		r.log.Info("pvc created", "namespace", opeaCache.Namespace, "name", opeaCache.Name)

		opeaCache.Status.Phase = opeaoperatorv1alpha1.OpeaCachePhaseExecuting

	}
	return nil

}

func (r *OpeaCacheReconciler) constructPVC(opeaCache *opeaoperatorv1alpha1.OpeaCache) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opeaCache.GetPVCName(),
			Namespace: opeaCache.Namespace,
		},
		Spec: opeaCache.Spec.Storage.PVC.PersistentVolumeClaimSpec,
	}
}

func (r *OpeaCacheReconciler) reconcileJob(ctx context.Context, opeaCache *opeaoperatorv1alpha1.OpeaCache) (res ctrl.Result, err error) {
	// Check if the download jobs already exists
	downloadJobs := &batchv1.Job{}
	downloadJobs.Name = fmt.Sprintf(downloadJobName, opeaCache.Name)
	downloadJobs.Namespace = opeaCache.Namespace
	if err = r.Get(ctx, client.ObjectKeyFromObject(downloadJobs), downloadJobs); err != nil && client.IgnoreNotFound(err) != nil {
		r.log.Error(err, "unable to get download job", "namespace", opeaCache.Namespace, "name", opeaCache.Name)
		return ctrl.Result{}, err
	} else if err != nil {
		// define a new jo
		job := newDownloadJob(opeaCache)
		if err = ctrl.SetControllerReference(opeaCache, job, r.scheme); err != nil {
			r.log.Error(err, "unable to set controller reference for download job")
			return ctrl.Result{}, err
		}
		if err = r.Create(ctx, job); err != nil {
			r.log.Error(err, "unable to create download job")
			return ctrl.Result{}, err
		}
		r.log.Info("download job created", "namespace", opeaCache.Namespace, "name", opeaCache.Name)
		return ctrl.Result{}, err
	} else {
		// job already exists
		r.log.Info("download job already exists")
		// get the pod of the job and check the status
		// find the pod for the job by labels: job-name
		podList := &v1.PodList{}
		podLabels := client.MatchingLabels{"job-name": downloadJobs.Name}
		if err = r.List(ctx, podList, client.InNamespace(opeaCache.Namespace), podLabels); err != nil {
			r.log.Error(err, "unable to list pods for job", "namespace", opeaCache.Namespace, "name", opeaCache.Name)
			return ctrl.Result{}, err
		}
		if len(podList.Items) == 0 {
			r.log.Error(fmt.Errorf("no pods found for job"), "namespace", opeaCache.Namespace, "name", opeaCache.Name)
			return ctrl.Result{}, fmt.Errorf("no pods found for job")
		}
		pod := &podList.Items[0]

		// if the job is finished, update the status of the cache
		// if the job is not finished, do nothing

		switch pod.Status.Phase {
		case v1.PodPending:
			opeaCache.Status.Phase = opeaoperatorv1alpha1.OpeaCachePhasePending
			if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Waiting != nil {
				err = fmt.Errorf("pod pending: %s", pod.Status.ContainerStatuses[0].State.Waiting.Reason)
			} else {
				err = fmt.Errorf("pod is pending, please check the pod")
			}
			res = ctrl.Result{RequeueAfter: 10 * time.Second}
		case v1.PodRunning:
			opeaCache.Status.Phase = opeaoperatorv1alpha1.OpeaCachePhaseExecuting
			res = ctrl.Result{RequeueAfter: 10 * time.Second}
			opeaCache.Status.CacheStates.CachedPercentage = "0%"
		case v1.PodUnknown:
			opeaCache.Status.Phase = opeaoperatorv1alpha1.OpeaCachePhaseExecuting
			res = ctrl.Result{RequeueAfter: 10 * time.Second}
		case v1.PodSucceeded:
			opeaCache.Status.Phase = opeaoperatorv1alpha1.OpeaCachePhaseReady
			// calculate the cache size
			opeaCache.Status.CacheStates.Cached = opeaCache.Spec.Storage.PVC.PersistentVolumeClaimSpec.Resources.Requests[v1.ResourceStorage]
			opeaCache.Status.CacheStates.CachedPercentage = "100%"
		case v1.PodFailed:
			opeaCache.Status.Phase = opeaoperatorv1alpha1.OpeaCachePhaseFailed
			if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
				err = fmt.Errorf("pod failed: %s", pod.Status.ContainerStatuses[0].State.Terminated.Reason)
			} else {
				err = fmt.Errorf("pod is failed, please check the pod")
			}
			res = ctrl.Result{RequeueAfter: 10 * time.Second}
		default:
			opeaCache.Status.Phase = opeaoperatorv1alpha1.OpeaCachePhaseExecuting
			res = ctrl.Result{RequeueAfter: 10 * time.Second}

		}
		r.log.Info("pod status", "namespace", opeaCache.Namespace, "name", opeaCache.Name,
			"pod status", pod.Status.Phase, "opeaCache status", opeaCache.Status.Phase)

		if err := r.Status().Update(ctx, opeaCache); err != nil {
			r.log.Error(err, "unable to update OpeaCache status:")
			return res, err
		}

	}

	return res, err
}

func newDownloadJob(opeaCache *opeaoperatorv1alpha1.OpeaCache) *batchv1.Job {
	var backoffLimit int32 = 3

	cmd, envs, err := constructCmd(opeaCache)
	if err != nil {
		log.Log.Error(err, "unable to construct cmd")
		return nil
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(downloadJobName, opeaCache.Name),
			Namespace: opeaCache.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:      "downloader",
							Image:     downloadJobImage,
							Command:   cmd,
							Env:       append(opeaCache.Spec.JobConf.Env, envs...),
							Resources: opeaCache.Spec.JobConf.Resources,
							VolumeMounts: []v1.VolumeMount{
								{MountPath: "/data", Name: opeaCache.GetPVCName()},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: opeaCache.GetPVCName(),
							VolumeSource: v1.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
									ClaimName: opeaCache.GetPVCName(),
								},
							},
						},
					},
					RestartPolicy: "OnFailure",
				},
			},
			PodReplacementPolicy: nil,
			ManagedBy:            &opeaCache.Name,
		},
	}

}
func constructCmd(opeaCache *opeaoperatorv1alpha1.OpeaCache) ([]string, []v1.EnvVar, error) {

	for _, source := range opeaCache.Spec.Sources {
		switch source.From {
		case "huggingface":
			return constructHFCmd(source)
		default:
			return nil, nil, nil
		}
	}

	return nil, nil, nil
}

func constructHFCmd(modelHub opeaoperatorv1alpha1.ModelHub) (cmds []string, envs []v1.EnvVar, err error) {
	hf := &opeaoperatorv1alpha1.HuggingFaceModel{}
	for _, val := range modelHub.Params {
		switch val.Name {
		case "repoId":
			hf.RepoID = val.Value
			if hf.RepoID == "" {
				return nil, nil, fmt.Errorf("repoId is required")
			}
		case "repoType":
			hf.RepoType = val.Value
			if hf.RepoType == "" {
				hf.RepoType = "model"
			}
		case "fileNames":
			hf.FileNames = strings.Split(val.Value, ",")

		case "token":
			if val.ValueFrom != nil {
				if val.ValueFrom.SecretKeyRef != nil {
					hf.Token = "$(token)"
					envs = append(envs, val)
				}
			} else {
				hf.Token = val.Value
			}

		}
	}
	hfStr, err := json.Marshal(hf)
	if err != nil {
		fmt.Print(err, "unable to marshal huggingface model")
		return nil, nil, err
	}

	cmds = []string{"opea-downloader", "huggingface", "--json", fmt.Sprintf("[%s]", string(hfStr))}
	return cmds, envs, nil
}

func isJobFinished(job *opeaoperatorv1alpha1.OpeaCache) bool {
	return true
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpeaCacheReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("opeacache-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&opeaoperatorv1alpha1.OpeaCache{}).
		Named("opeacache").
		Complete(r)
}
