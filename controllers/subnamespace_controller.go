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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	danav1alpha1 "github.com/Dana-Team/SNS/api/v1alpha1"
	"github.com/go-logr/logr"
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
	var subspace danav1alpha1.Subnamespace
	var ownerNamespace v1.Namespace
	var childNamespace v1.Namespace
	var childNamespaceName string

	// Getting reconciled subspace
	if err := r.Get(ctx, req.NamespacedName, &subspace); err != nil {
		log.Error(err, "Could not find Subspace")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Getting subspace's namespace - described as ownerNamespace
	if err := r.Get(ctx, client.ObjectKey{Namespace: "", Name: subspace.GetNamespace()}, &ownerNamespace); err != nil {
		log.Error(err, "Could not find owner Namespace ")
		return ctrl.Result{}, err
	}

	// Hierarchy 'breadcrumb' name, built from ownerNamespace and subspace.
	childNamespaceName = ownerNamespace.Name + "-" + subspace.Name

	// current subspace's phase flow
	switch currentPhase := subspace.Status.Phase; currentPhase {
	case danav1alpha1.None:
		if err := r.InitializeSubspace(ctx, ownerNamespace, subspace, childNamespaceName); err != nil {
			log.Error(err, "Could not update Subspace Phase from 'None' to 'Missing'")

		}
	case danav1alpha1.Missing:
		if err := r.Get(ctx, client.ObjectKey{Namespace: "", Name: childNamespaceName}, &childNamespace); err != nil {
			if errors.IsNotFound(err) {
				// Child Namespace is not found, therefore we are creating one
				if requeueResult, err := r.CreateNamespace(ctx, childNamespace, ownerNamespace, subspace); err != nil {
					log.Error(err, "Could not create Namespace ")
					return ctrl.Result{}, err
				} else {
					return ctrl.Result{Requeue: requeueResult}, nil
				}
			}
		} else {
			// Child Namespace is exist, so we are changing the Subspace phase to created
			if _, err := ctrl.CreateOrUpdate(ctx, r.Client, &subspace, func() error {
				subspace.Status.Phase = danav1alpha1.Created
				return nil
			}); err != nil {
				log.Error(err, "Could not update Subspace Phase from 'Missing' to 'Created'")
			}
		}

	case danav1alpha1.Created:
		if err := r.Get(ctx, client.ObjectKey{Namespace: "", Name: childNamespaceName}, &childNamespace); err != nil {
			if errors.IsNotFound(err) {
				if _, err := ctrl.CreateOrUpdate(ctx, r.Client, &subspace, func() error {
					subspace.Status.Phase = danav1alpha1.Missing
					return nil
				}); err != nil {
					log.Error(err, "Could not update Subspace Phase from 'Created' to 'Missing'")
				}
			}
		}

	}
	return ctrl.Result{}, nil
}
func GetNewChildNamespace(ownerNamespace v1.Namespace, subspace danav1alpha1.Subnamespace) v1.Namespace {
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

func GetChildNamespaceName(ownerNamespace v1.Namespace, subspace danav1alpha1.Subnamespace) string {
	return ownerNamespace.Name + "-" + subspace.Name
}
func (r *SubnamespaceReconciler) CreateNamespace(ctx context.Context, childNamespace v1.Namespace, ownerNamespace v1.Namespace, subspace danav1alpha1.Subnamespace) (requeueResult bool, err error) {
	childNamespace = GetNewChildNamespace(ownerNamespace, subspace)
	if err := r.Create(ctx, &childNamespace); err != nil {
		return false, err
	} else {
		return true, nil
	}
}

func (r *SubnamespaceReconciler) InitializeSubspace(ctx context.Context, ownerNamespace v1.Namespace, subspace danav1alpha1.Subnamespace, childNamespaceName string) error {
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, &subspace, func() error {
		subspace.SetAnnotations(map[string]string{
			danav1alpha1.Pointer: childNamespaceName,
		})
		subspace.Status.Phase = danav1alpha1.Missing
		if err := ctrl.SetControllerReference(&ownerNamespace, &subspace, r.Scheme); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil

}
func (r *SubnamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&danav1alpha1.Subnamespace{}).
		Complete(r)
}
