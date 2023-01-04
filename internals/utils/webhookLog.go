package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"os"
	"strconv"
	"time"
)

var (
	server    = os.Getenv("ELASTIC_SERVER")
	port, _   = strconv.Atoi(os.Getenv("ELASTIC_PORT"))
	indexName = os.Getenv("ELASTIC_INDEX")
)

type elasticSearch struct {
	server    string
	port      int
	indexName string
}

func (elk *elasticSearch) GetElasticIndexUrl() string {

	return fmt.Sprintf("http://%s:%d/%s-%d-%d/doc", elk.server, elk.port, elk.indexName, time.Now().Year(), int(time.Now().Month()))
}

type Action string

// WebhookLog Actions
const (
	Create Action = "create"
	Delete Action = "delete"
	Edit   Action = "edit"
)

type WebhookLog struct {
	Timestamp  string `json:"timestamp"`
	ObjectType string `json:"object_type"`
	Resources  string `json:"resources"`
	Namespace  string `json:"namespace"`
	Action     Action `json:"action"`
	User       string `json:"user"`
	Cluster    string `json:"cluster"`
	Message    string `json:"message"`
	elastic    elasticSearch
}

// NewWebhookLog Creating new objectContext object
func NewWebhookLog(timestamp time.Time, objectType string, namespace string, action Action, user string, message string, resources corev1.ResourceList, cluster string) *WebhookLog {
	return &WebhookLog{
		Timestamp:  timestamp.Format("2006-01-02T15:04:05.000000"),
		ObjectType: objectType,
		Resources:  resourceListToString(resources),
		Namespace:  namespace,
		Action:     action,
		User:       user,
		Cluster:    cluster,
		Message:    message,
		elastic: elasticSearch{
			server:    server,
			port:      port,
			indexName: indexName,
		},
	}
}

func (log *WebhookLog) UploadLogToElastic() error {
	jsonValues, _ := json.Marshal(log)
	req, err := http.NewRequest("POST", log.elastic.GetElasticIndexUrl(), bytes.NewBuffer(jsonValues))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Timeout: 1 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func resourceListToString(resourceList corev1.ResourceList) string {
	b := new(bytes.Buffer)
	for resourceName, quantity := range resourceList {
		_, err := fmt.Fprintf(b, "%s=\"%s\"\n", resourceName, quantity.String())
		if err != nil {
			return ""
		}
	}
	return b.String()
}
