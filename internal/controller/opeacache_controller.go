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
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	opeaoperatorv1alpha1 "github.com/opea-project/GenAIInfra/opea-operator/api/v1alpha1"
)

const jobOwnerKey = "opea-cache.controller"

// OpeaCacheReconciler reconciles a OpeaCache object
type OpeaCacheReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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

	logger := log.FromContext(ctx)
	logger.Info("Reconciling OpeaCache:", "namespace", req.Namespace, "name", req.Name)

	// Fetch the OpeaCache instance
	opeaCache := &opeaoperatorv1alpha1.OpeaCache{}
	if err := r.Get(ctx, req.NamespacedName, opeaCache); err != nil {
		logger.Error(err, "unable to fetch OpeaCache", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check if the download jobs already exists
	downloadJobs := &batchv1.JobList{}
	if err := r.List(ctx, downloadJobs, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
		logger.Error(err, "unable to list download jobs", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, err
	}
	if len(downloadJobs.Items) == 0 {
		// define a new job
		job := newDownloadJob(opeaCache)
		if err := ctrl.SetControllerReference(opeaCache, job, r.Scheme); err != nil {
			logger.Error(err, "unable to set controller reference for download job")
			return ctrl.Result{}, err
		}

		// create the job
		if err := r.Create(ctx, job); err != nil {
			logger.Error(err, "unable to create download job")
			return ctrl.Result{}, err
		}

		// update the status of the opeaCache to executing
		opeaCache.Status.Phase = opeaoperatorv1alpha1.OpeaCachePhaseExecuting
		if err := r.Status().Update(ctx, opeaCache); err != nil {
			logger.Error(err, "unable to update OpeaCache status")
			return ctrl.Result{}, err
		}
	} else {
		// job already exists
		logger.Info("download job already exists")

		// update the opeaCache status by the job status
		job := downloadJobs.Items[0]

		switch job.Status.Succeeded {

		default:
			logger.Info(" check job status")
		}

	}

	if isJobFinished(opeaCache) {
		// update opeaCache status

		return ctrl.Result{}, nil
	} else {

		//

		logger.Info("will check the cache status next time")
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

}

func newDownloadJob(opeaCache *opeaoperatorv1alpha1.OpeaCache) *batchv1.Job {
	return &batchv1.Job{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       batchv1.JobSpec{},
		Status:     batchv1.JobStatus{},
	}
}

func isJobFinished(job *opeaoperatorv1alpha1.OpeaCache) bool {
	if job.Status.Phase == opeaoperatorv1alpha1.OpeaCachePhaseSucceeded {
		return true
	} else {
		return false
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpeaCacheReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opeaoperatorv1alpha1.OpeaCache{}).
		Named("opeacache").
		Complete(r)
}
