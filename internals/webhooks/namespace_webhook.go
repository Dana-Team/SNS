package webhooks

import (
	"context"
	"fmt"
	"github.com/Dana-Team/SNS/internals/api/v1alpha1"
	"github.com/prometheus/common/log"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type NamepsaceAnnotator struct {
	Client  client.Client
	Decoder *admission.Decoder
}

// +kubebuilder:webhook:path=/validate-v1-namespace,mutating=false,failurePolicy=fail,groups="core",resources=namespaces,verbs=delete,versions=v1,name=dana.nhs.io

func (a *NamepsaceAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	namespace := &corev1.Namespace{}

	if err := a.Decoder.DecodeRaw(req.OldObject, namespace); err != nil {
		log.Error(err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := IsAnnotationMatch(v1alpha1.Role, v1alpha1.Leaf, namespace); err != nil {
		log.Info("annotation didn't match")
		return admission.Denied(fmt.Sprintf(denyMessage))

	}
	log.Info("annotation matched")
	return admission.Allowed(allowMessage)
}

func IsAnnotationMatch(annotationKey string, annotationValue string, object client.Object) error {
	objectAnnotations := object.GetAnnotations()
	value, found := objectAnnotations[annotationKey]
	if !found {
		return AnnotationNotFoundError(annotationKey)
	}
	if value != annotationValue {
		return AnnotationValueError(annotationKey, annotationValue)
	}
	return nil
}

func (a *NamepsaceAnnotator) InjectDecoder(d *admission.Decoder) error {
	a.Decoder = d
	return nil
}
