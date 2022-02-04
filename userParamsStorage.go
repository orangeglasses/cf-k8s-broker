package main

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
)

func paramsCMName(instanceID string) string {
	return fmt.Sprintf("params-%s", instanceID)
}

func (k *k8sclient) StoreUserParams(ctx context.Context, instanceID string, params map[string]interface{}) error {
	paramsJson, err := json.Marshal(params)
	if err != nil {
		return err
	}

	cm := corev1.ConfigMap(paramsCMName(instanceID), "default")
	cm.Data = map[string]string{
		"params": string(paramsJson),
	}
	_, err = k.clientSet.CoreV1().ConfigMaps("default").Apply(ctx, cm, metav1.ApplyOptions{FieldManager: "cf-k8s-broker"})
	return err
}

func (k *k8sclient) GetPreviousUserParams(ctx context.Context, instanceID string) map[string]interface{} {
	params := make(map[string]interface{})

	obj, err := k.clientSet.CoreV1().ConfigMaps("default").Get(ctx, paramsCMName(instanceID), metav1.GetOptions{})
	if err == nil && obj.Data != nil {
		if paramsJson, ok := obj.Data["params"]; ok {
			json.Unmarshal([]byte(paramsJson), &params)
		}
	}

	return params
}

func (k *k8sclient) DeleteStoredUserParams(ctx context.Context, instanceID string) error {
	return k.clientSet.CoreV1().ConfigMaps("default").Delete(ctx, paramsCMName(instanceID), metav1.DeleteOptions{})
}
