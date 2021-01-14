package v1alpha1

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type NamepsaceAnnotator struct {
	Client  client.Client
	Decoder *admission.Decoder
}

// +kubebuilder:webhook:path=/validate-v1-namespace,mutating=false,failurePolicy=fail,groups="core",resources=namespaces,verbs=create;update,versions=v1,name=dana.nhs.io

func (a *NamepsaceAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	denyMessage := "Delete non-leaf namespace is forbidden"
	allowMessage := "Deleted successfully"
	namespace := &corev1.Namespace{}

	err := a.Decoder.Decode(req, namespace)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	value, found := namespace.Annotations[Role]
	if !found || value != Leaf {
		return admission.Denied(fmt.Sprintf(denyMessage))
	}

	return admission.Allowed(allowMessage)
}
func (a *NamepsaceAnnotator) InjectDecoder(d *admission.Decoder) error {
	a.Decoder = d
	return nil
}
