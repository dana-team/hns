package webhooks

import (
	"errors"
)

func AnnotationNotFoundError(annotationKey string) error {
	return errors.New("annotation key :" + annotationKey + "not found.")
}

func AnnotationValueError(annotationKey string, annotationValue string) error {
	return errors.New("annotation key: " + annotationKey + "not matched value: " + annotationValue)
}

func LabelNotFoundError(labelKey string) error {
	return errors.New("label :" + labelKey + "not found.")
}
