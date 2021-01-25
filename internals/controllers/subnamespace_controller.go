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
	"strconv"
	"strings"

	danav1alpha1 "github.com/Dana-Team/SNS/api/v1alpha1"
	"github.com/go-logr/logr"
	openshiftv1 "github.com/openshift/api/quota/v1"
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
		shouldCreateChildNs, err := r.IsObjectMissing(ctx, log, &v1.Namespace{}, "", childNamespaceName)
		if err != nil {
			return ctrl.Result{}, err
		}

		if shouldCreateChildNs {
			// Namespace not exist, creating it
			if err := r.CreateNamespace(ctx, log, &ownerNamespace, &subspace); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("child namespace created, requeue")
			return ctrl.Result{Requeue: true}, nil
		} else {
			// Namespace exist, now checking CRQ existence
			shouldCreateCRQ, err := r.IsObjectMissing(ctx, log, &openshiftv1.ClusterResourceQuota{}, "", childNamespaceName)
			if err != nil {
				return ctrl.Result{}, err
			}

			if shouldCreateCRQ {
				// CRQ not exist, creating it
				if err := r.CreateCRQ(ctx, log, &ownerNamespace, &subspace); err != nil {
					return ctrl.Result{}, err
				}
			}
			if err := r.UpdateSubspacePhase(ctx, log, &subspace, danav1alpha1.Created); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("subnamespace phase updated from Missing to Created")
			return ctrl.Result{}, nil
		}

	case danav1alpha1.Created:
		var subspaceCRQ openshiftv1.ClusterResourceQuota
		if err := r.UpdateCRQ(ctx, log, &subspaceCRQ, &subspace, childNamespaceName); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil

	default:

		err := errors.New("unexpected subnamespace phase")
		log.Error(err, "unexpected subnamespace phase")
		return ctrl.Result{}, err

	}

}

func (r *SubnamespaceReconciler) IsObjectMissing(ctx context.Context, log logr.Logger, object client.Object, namespaceName string, objectName string) (bool, error) {
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: objectName}, object); err != nil {
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
	ownerNamespaceDepth, err := strconv.Atoi(ownerNamespace.Annotations[danav1alpha1.Depth])
	if err != nil {
		// Only for subnamespace_controller.go migration
		ownerNamespaceDepth = 1
	}
	childNamespaceDepth := strconv.Itoa(ownerNamespaceDepth + 1)

	childNamespaceName := GetChildNamespaceName(ownerNamespace, subspace)
	childNamespace := &v1.Namespace{
		ObjectMeta: v1api.ObjectMeta{
			Name: childNamespaceName,
			Labels: map[string]string{
				danav1alpha1.Hns:    "true",
				danav1alpha1.Parent: ownerNamespace.Name,
			},
			Annotations: map[string]string{
				danav1alpha1.Depth:      childNamespaceDepth,
				danav1alpha1.SnsPointer: subspace.Name,
				danav1alpha1.CrqSelector + "-" + childNamespaceDepth: childNamespaceName,
			},
		},
	}
	AppendAnnotationsToNamespace(ownerNamespace, childNamespace)
	return *childNamespace
}

func GetNewChildCRQ(ownerNamespace *v1.Namespace, subspace *danav1alpha1.Subnamespace) openshiftv1.ClusterResourceQuota {
	childCRQName := GetChildNamespaceName(ownerNamespace, subspace)
	childCRQ := &openshiftv1.ClusterResourceQuota{
		ObjectMeta: v1api.ObjectMeta{
			Name: childCRQName,
		},
		Spec: openshiftv1.ClusterResourceQuotaSpec{
			Selector: openshiftv1.ClusterResourceQuotaSelector{
				AnnotationSelector: nil,
			},
			Quota: subspace.Spec.ResourceQuotaSpec,
		},
	}
	SetCrqSelectorAnnotations(ownerNamespace, childCRQ, childCRQName)
	return *childCRQ
}

func AppendAnnotationsToNamespace(parentNamespace *v1.Namespace, childNamespace *v1.Namespace) {
	existingAnnotations := childNamespace.GetAnnotations()
	for key, value := range parentNamespace.GetAnnotations() {
		if strings.Contains(key, danav1alpha1.CrqSelector) {
			existingAnnotations[key] = value
		}
	}
	childNamespace.SetAnnotations(existingAnnotations)
}

func SetCrqSelectorAnnotations(ownerNamespace *v1.Namespace, childCRQ *openshiftv1.ClusterResourceQuota, childCRQName string) {
	if childCRQ.Spec.Selector.AnnotationSelector == nil {
		childCRQ.Spec.Selector.AnnotationSelector = make(map[string]string)
	}
	ownerDepth, _ := strconv.Atoi(ownerNamespace.Annotations[danav1alpha1.Depth])
	childCRQ.Spec.Selector.AnnotationSelector[danav1alpha1.CrqSelector+"-"+strconv.Itoa(ownerDepth+1)] = childCRQName
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

func (r *SubnamespaceReconciler) CreateCRQ(ctx context.Context, log logr.Logger, ownerNamespace *v1.Namespace, subspace *danav1alpha1.Subnamespace) error {
	childCRQ := GetNewChildCRQ(ownerNamespace, subspace)
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, &childCRQ, func() error {
		return nil
	}); err != nil {
		log.Error(err, "Could not create child namespace")
		return err
	}
	return nil

}

func (r *SubnamespaceReconciler) UpdateCRQ(ctx context.Context, log logr.Logger, crqToUpdate *openshiftv1.ClusterResourceQuota, subspace *danav1alpha1.Subnamespace, crqName string) error {
	if err := r.Get(ctx, client.ObjectKey{Name: crqName}, crqToUpdate); err != nil {
		log.Error(err, "Could not find CRQ")
		return err
	}
	crqToUpdate.Spec.Quota = subspace.Spec.ResourceQuotaSpec
	if err := r.Update(ctx, crqToUpdate); err != nil {
		log.Error(err, "Could not update CRQ")
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
	if err := openshiftv1.AddToScheme(mgr.GetScheme()); err != nil {

	}

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
