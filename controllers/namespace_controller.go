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
	"github.com/go-logr/logr"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	danav1alpha1 "github.com/Dana-Team/SNS/api/v1alpha1"
)

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type NamespacePredicate struct {
	predicate.Funcs
}

// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces/status,verbs=get;update;patch

func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("namespace", req.NamespacedName)
	log.Info("starting to reconcile")

	var ns corev1.Namespace

	if err := r.Get(ctx, req.NamespacedName, &ns); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("deleted")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		log.Error(err, "unable to get Namespace")
		return ctrl.Result{}, err
	}

	//ns is being deleted
	if !ns.DeletionTimestamp.IsZero() {
		shouldRequeue, err := r.CleanUp(ctx, log, &ns)
		return ctrl.Result{
			Requeue: shouldRequeue,
		}, err
	}

	if controllerutil.ContainsFinalizer(&ns, danav1alpha1.NsFinalizer) {
		err := r.Update(ctx, log, &ns)
		return ctrl.Result{}, err
	}

	err := r.Init(ctx, log, &ns)
	return ctrl.Result{}, err
}

func (r *NamespaceReconciler) Init(ctx context.Context, log logr.Logger, namespace *corev1.Namespace) error {
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, namespace, func() error {
		controllerutil.AddFinalizer(namespace, danav1alpha1.NsFinalizer)
		return nil
	}); err != nil {
		if apierrors.IsConflict(err) {
			log.Info("newer resource version exists")
			return nil
		}
		log.Error(err, "unable to add finalizer")
		return err
	}
	log.Info("finalizer created")
	return nil
}

func (r *NamespaceReconciler) Update(ctx context.Context, log logr.Logger, namespace *corev1.Namespace) error {
	var snsList danav1alpha1.SubnamespaceList
	role := danav1alpha1.NoRole

	if err := r.List(ctx, &snsList, client.InNamespace(namespace.Name)); err != nil {
		return err
	}

	if len(snsList.Items) == 0 {
		role = danav1alpha1.Leaf
	}

	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, namespace, func() error {
		namespace.Annotations[danav1alpha1.Role] = role
		return nil
	}); err != nil {
		log.Error(err, "unable to update role")
		return err
	}

	return nil
}

func (r *NamespaceReconciler) CleanUp(ctx context.Context, log logr.Logger, namespace *corev1.Namespace) (shouldRequeue bool, err error) {
	var sns danav1alpha1.Subnamespace

	//check if sns is gone
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: namespace.Labels[danav1alpha1.Parent],
		Name:      namespace.Annotations[danav1alpha1.SnsPointer],
	}, &sns); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "unable to get Subnamespace")
			return false, err
		}
	} else {
		//sns still present
		if err := r.Delete(ctx, &sns); err != nil {
			log.Error(err, "unable to delete Subnamespace")
			return false, err
		}
		log.Info("sns deleted")
		return true, nil
	}
	//sns is gone did we already remove the finalizer
	if !controllerutil.ContainsFinalizer(namespace, danav1alpha1.NsFinalizer) {
		return false, nil
	}
	//remove finalizer
	if _, err := ctrl.CreateOrUpdate(ctx, r.Client, namespace, func() error {
		controllerutil.RemoveFinalizer(namespace, danav1alpha1.NsFinalizer)
		return nil

	}); err != nil {
		log.Error(err, "unable to remove finalizer")
		return false, err
	}
	log.Info("finalizer removed")
	return false, nil
}

func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		//filter all namespace without the HNS label
		WithEventFilter(NamespacePredicate{predicate.NewPredicateFuncs(func(object client.Object) bool {
			//always allow reconcile for subnamespaces
			if reflect.TypeOf(object) == reflect.TypeOf(&danav1alpha1.Subnamespace{}) {
				return true
			}
			objLabels := object.GetLabels()
			if _, ok := objLabels[danav1alpha1.Hns]; ok {
				return true
			}
			return false
		})}).
		//also reconcile when subnamespace is changed
		Owns(&danav1alpha1.Subnamespace{}).
		Complete(r)
}
