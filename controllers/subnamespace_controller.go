/*


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

package controllers

import (
	"context"
	"errors"

	danav1alpha1 "github.com/Dana-Team/SNS/api/v1alpha1"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1api "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SubnamespaceReconciler reconciles a Subnamespace object
type SubnamespaceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=dana.dana.hns.io,resources=subnamespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dana.dana.hns.io,resources=subnamespaces/status,verbs=get;update;patch

func (r *SubnamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("subspace", req.NamespacedName)
	var (
		ownerNamespace v1.Namespace
		subspace       danav1alpha1.Subnamespace
	)

	log.Info("starting to reconcile")

	// Getting reconciled subspace
	if err := r.Get(ctx, req.NamespacedName, &subspace); err != nil {
		log.Info("Subspace deleted")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Getting subspace's namespace - described as ownerNamespace
	if err := r.Get(ctx, client.ObjectKey{Namespace: "", Name: subspace.GetNamespace()}, &ownerNamespace); err != nil {
		log.Error(err, "Could not find owner Namespace ")
		return ctrl.Result{}, err
	}

	// Hierarchy 'breadcrumb' name, built from ownerNamespace and subspace.
	childNamespaceName := GetChildNamespaceName(&ownerNamespace, &subspace)

	// current subspace's phase flow
	switch currentPhase := subspace.Status.Phase; currentPhase {
	case danav1alpha1.None:

		if err := r.InitSubspace(ctx, log, &ownerNamespace, &subspace, childNamespaceName); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Subspace inited")
		return ctrl.Result{}, nil

	case danav1alpha1.Missing:

		shouldCreateChildNs, err := r.IsChildNsMissing(ctx, log, childNamespaceName)
		if err != nil {
			return ctrl.Result{}, err
		}

		if shouldCreateChildNs {
			if err := r.CreateNamespace(ctx, log, &ownerNamespace, &subspace); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("child namespace created")
			return ctrl.Result{Requeue: true}, nil
		} else {
			if err := r.UpdateSubspacePhase(ctx, log, &subspace, danav1alpha1.Created); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("subnamespace phase updated from Missing to Created")
			return ctrl.Result{}, nil
		}

	case danav1alpha1.Created:

		shouldUpdateSnsPhase, err := r.IsChildNsMissing(ctx, log, childNamespaceName)
		if err != nil {
			return ctrl.Result{}, err
		}
		if shouldUpdateSnsPhase {
			if err := r.UpdateSubspacePhase(ctx, log, &subspace, danav1alpha1.Missing); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("subnamespace phase updated from Created to Missing")

			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, nil
		}

	default:

		err := errors.New("unexpected subnamespace phase")
		log.Error(err, "unexpected subnamespace phase")
		return ctrl.Result{}, err

	}

}

func (r *SubnamespaceReconciler) IsChildNsMissing(ctx context.Context, log logr.Logger, childName string) (bool, error) {
	if err := r.Get(ctx, client.ObjectKey{Namespace: "", Name: childName}, &v1.Namespace{}); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		} else {
			log.Error(err, "Could not get child namespace")
			return false, err
		}
	}
	return false, nil
}

func GetNewChildNamespace(ownerNamespace *v1.Namespace, subspace *danav1alpha1.Subnamespace) v1.Namespace {
	childNamespaceName := GetChildNamespaceName(ownerNamespace, subspace)
	return v1.Namespace{
		ObjectMeta: v1api.ObjectMeta{
			Name: childNamespaceName,
			Labels: map[string]string{
				danav1alpha1.Hns:    "true",
				danav1alpha1.Parent: ownerNamespace.Name,
			},
			Annotations: map[string]string{
				danav1alpha1.SnsPointer: subspace.Name,
			},
		},
	}
}

func GetChildNamespaceName(ownerNamespace *v1.Namespace, subspace *danav1alpha1.Subnamespace) string {
	return ownerNamespace.Name + "-" + subspace.Name
}
func (r *SubnamespaceReconciler) CreateNamespace(ctx context.Context, log logr.Logger, ownerNamespace *v1.Namespace, subspace *danav1alpha1.Subnamespace) error {
	var childNamespace = GetNewChildNamespace(ownerNamespace, subspace)
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, &childNamespace, func() error {
		return nil
	}); err != nil {
		log.Error(err, "Could not create child namespace")
		return err
	}
	return nil

}

func (r *SubnamespaceReconciler) InitSubspace(ctx context.Context, log logr.Logger, ownerNamespace *v1.Namespace, subspace *danav1alpha1.Subnamespace, childNamespaceName string) error {
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, subspace, func() error {
		subspace.SetAnnotations(map[string]string{
			danav1alpha1.Pointer: childNamespaceName,
		})
		subspace.Status.Phase = danav1alpha1.Missing
		if err := ctrl.SetControllerReference(ownerNamespace, subspace, r.Scheme); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if !apierrors.IsConflict(err) {
			log.Error(err, "Could not update Subspace Phase from 'None' to 'Missing'")
			return err
		}
		log.Info("newer resource version exists")
	}
	return nil

}
func (r *SubnamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&danav1alpha1.Subnamespace{}).
		Complete(r)
}

func (r *SubnamespaceReconciler) UpdateSubspacePhase(ctx context.Context, log logr.Logger, subspace *danav1alpha1.Subnamespace, phase danav1alpha1.Phase) error {
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, subspace, func() error {
		subspace.Status.Phase = phase
		return nil
	}); err != nil {
		if !apierrors.IsConflict(err) {
			log.Error(err, "Could not update Subspace Phase from 'Missing' to 'Created'")
			return err
		}
		log.Info("newer resource version exists")
	}
	return nil
}
