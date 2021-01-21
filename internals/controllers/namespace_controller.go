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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	log.V(1).Info("starting to reconcile")

	var ns corev1.Namespace

	//get namespace instance
	if err := r.Get(ctx, req.NamespacedName, &ns); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(3).Info("deleted")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		log.V(4).Error(err, "unable to get Namespace")
		return ctrl.Result{}, err
	}

	//we dont want to reconcile on root namespace since there is no need to init, sync or cleanup after them
	//also we must not add finalizer to root namespace since we wont be able to be deleted
	if isRoot(&ns) {
		log.V(2).Info("is root, skip")
		return ctrl.Result{}, nil
	}

	//ns is being deleted
	if shouldCleanUp(&ns) {
		log.V(2).Info("straing to clean up")
		return ctrl.Result{}, r.CleanUp(ctx, log, &ns)
	}

	if shouldSync(&ns) {
		log.V(2).Info("straing to sync")
		return ctrl.Result{}, r.Sync(ctx, log, &ns)
	}

	log.V(2).Info("straing to init")
	return ctrl.Result{}, r.Init(ctx, log, &ns)
}

//Init is being called at the first time namesapce is be reconciled and adds a finalizer to it
func (r *NamespaceReconciler) Init(ctx context.Context, log logr.Logger, namespace *corev1.Namespace) error {

	controllerutil.AddFinalizer(namespace, danav1alpha1.NsFinalizer)
	if err := r.Update(ctx, namespace); err != nil {
		if apierrors.IsConflict(err) {
			log.V(3).Info("newer resource version exists")
			return nil
		}
		log.V(4).Error(err, "unable to update namespace")
		return err
	}

	return nil
}

//Sync is being call every time there is an update in the namepsace's children and make sure its role is up to date
func (r *NamespaceReconciler) Sync(ctx context.Context, log logr.Logger, namespace *corev1.Namespace) error {

	var (
		role    = danav1alpha1.NoRole
		snsList = danav1alpha1.SubnamespaceList{}
	)

	if err := r.List(ctx, &snsList, client.InNamespace(namespace.Name)); err != nil {
		return err
	}

	//if ns has no sns then he is a leaf
	if len(snsList.Items) == 0 {
		role = danav1alpha1.Leaf
	}

	if namespace.Labels[danav1alpha1.Role] == role {
		return nil
	}

	namespace.Annotations[danav1alpha1.Role] = role
	if err := r.Update(ctx, namespace); err != nil {
		log.V(4).Error(err, "unable to update namespace")
		return err
	}
	log.V(3).Info("role updated")
	return nil
}

//CleanUp is being called when a namespace is being deleted,it deletes the subnamespace object related to the namespace inside its parent namespace,
//also removing the finalizer from the namespace so it could be deleted
func (r *NamespaceReconciler) CleanUp(ctx context.Context, log logr.Logger, namespace *corev1.Namespace) error {

	sns := getSns(namespace)
	if err := r.Delete(ctx, &sns); err != nil {
		//We should treat the case when the Delete did not find the subnamespace we are trying to delete
		//since we must make sure the finalizer is removed even though the subnamespace is missing
		if !apierrors.IsNotFound(err) {
			log.V(4).Error(err, "unable to delete Subnamespace")
			return err
		}
	}

	controllerutil.RemoveFinalizer(namespace, danav1alpha1.NsFinalizer)
	if err := r.Update(ctx, namespace); err != nil {
		log.V(4).Error(err, "unable to update namespace")
		return err
	}

	log.V(3).Info("cleaned up")
	return nil
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

func isRoot(namespace *corev1.Namespace) bool {
	return namespace.Annotations[danav1alpha1.Role] == danav1alpha1.Root
}

func shouldCleanUp(namespace *corev1.Namespace) bool {
	return !namespace.DeletionTimestamp.IsZero()
}

func shouldSync(namespace *corev1.Namespace) bool {
	return controllerutil.ContainsFinalizer(namespace, danav1alpha1.NsFinalizer)
}

func getSns(namespace *corev1.Namespace) danav1alpha1.Subnamespace {
	return danav1alpha1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace.Annotations[danav1alpha1.SnsPointer],
			Namespace: namespace.Labels[danav1alpha1.Parent],
		},
	}
}
