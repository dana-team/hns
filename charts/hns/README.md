# hns

![Version: 0.0.0](https://img.shields.io/badge/Version-0.0.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: latest](https://img.shields.io/badge/AppVersion-latest-informational?style=flat-square)

A Helm chart for the hns operator.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Node affinity rules for scheduling pods. Allows you to specify advanced node selection constraints. |
| fullnameOverride | string | `""` |  |
| hnsConfig.enabled | bool | `false` | create an HNSConfig resource to configure the HNS controller. |
| hnsConfig.limitRange | object | `{"defaultLimit":{"cpu":"150m","memory":"300Mi"},"defaultRequest":{"cpu":"50m","memory":"100Mi"},"maximum":{"cpu":128},"minimum":{"cpu":"25m","memory":"50Mi"},"minimumPVC":{"storage":"20Mi"}}` | Default values for the LimitRange created in each namespace. |
| hnsConfig.name | string | `"hns-config"` |  |
| hnsConfig.observedResources | list | `["basic.storageclass.storage.k8s.io/requests.storage","cpu","memory","pods","requests.nvidia.com/gpu"]` | Resources that the HNSConfig controller will manage. |
| hnsConfig.permittedGroups | list | `["test"]` | Groups that are allowed to create and manage HNSConfig resources. |
| image.manager.pullPolicy | string | `"IfNotPresent"` | The pull policy for the image. |
| image.manager.repository | string | `"ghcr.io/dana-team/hns"` | The repository of the manager container image. |
| image.manager.tag | string | `""` | The tag of the manager container image. |
| livenessProbe | object | `{"initialDelaySeconds":15,"periodSeconds":20,"port":8081}` | Configuration for the liveness probe. |
| livenessProbe.initialDelaySeconds | int | `15` | The initial delay before the liveness probe is initiated. |
| livenessProbe.periodSeconds | int | `20` | The frequency (in seconds) with which the probe will be performed. |
| livenessProbe.port | int | `8081` | The port for the health check endpoint. |
| manager | object | `{"args":["--leader-elect","--health-probe-bind-address=:8081","--metrics-bind-address=:8443","--max-sns=250"],"command":["/manager"],"ports":{"health":{"containerPort":8081,"name":"health","protocol":"TCP"},"https":{"containerPort":8443,"name":"https","protocol":"TCP"},"webhook":{"containerPort":9443,"name":"webhook-server","protocol":"TCP"}},"resources":{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"10m","memory":"64Mi"}},"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}},"volumeMounts":[{"mountPath":"/tmp/k8s-webhook-server/serving-certs","name":"cert","readOnly":true}],"webhookServer":{"defaultMode":420,"secretName":"webhook-server-cert"}}` | Configuration for the manager container. |
| manager.args | list | `["--leader-elect","--health-probe-bind-address=:8081","--metrics-bind-address=:8443","--max-sns=250"]` | Command-line arguments passed to the manager container. |
| manager.command | list | `["/manager"]` | Command-line commands passed to the manager container. |
| manager.ports | object | `{"health":{"containerPort":8081,"name":"health","protocol":"TCP"},"https":{"containerPort":8443,"name":"https","protocol":"TCP"},"webhook":{"containerPort":9443,"name":"webhook-server","protocol":"TCP"}}` | Port configurations for the manager container. |
| manager.ports.health.containerPort | int | `8081` | The port for the health check endpoint. |
| manager.ports.health.name | string | `"health"` | The name of the health check port. |
| manager.ports.health.protocol | string | `"TCP"` | The protocol used by the health check endpoint. |
| manager.ports.https.containerPort | int | `8443` | The port for the HTTPS endpoint. |
| manager.ports.https.name | string | `"https"` | The name of the HTTPS port. |
| manager.ports.https.protocol | string | `"TCP"` | The protocol used by the HTTPS endpoint. |
| manager.ports.webhook.containerPort | int | `9443` | The port for the webhook server. |
| manager.ports.webhook.name | string | `"webhook-server"` | The name of the webhook port. |
| manager.ports.webhook.protocol | string | `"TCP"` | The protocol used by the webhook server. |
| manager.resources | object | `{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"10m","memory":"64Mi"}}` | Resource requests and limits for the manager container. |
| manager.securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}` | Security settings for the manager container. |
| manager.volumeMounts | list | `[{"mountPath":"/tmp/k8s-webhook-server/serving-certs","name":"cert","readOnly":true}]` | Volume mounts for the manager container. |
| manager.webhookServer.defaultMode | int | `420` | The default mode for the secret. |
| manager.webhookServer.secretName | string | `"webhook-server-cert"` | The name of the secret containing the webhook server certificate. |
| monitoring | object | `{"enabled":false,"service":{"port":8443,"protocol":"TCP","targetPort":8443,"type":"ClusterIP"},"serviceMonitor":{"interval":"30s","labels":{},"metricRelabelings":[],"relabelings":[],"scrapeTimeout":"10s"}}` | Configuration for prometheus monitoring. |
| monitoring.enabled | bool | `false` | Enable or disable Prometheus monitoring. |
| monitoring.service | object | `{"port":8443,"protocol":"TCP","targetPort":8443,"type":"ClusterIP"}` | Configuration for the Prometheus service. |
| monitoring.serviceMonitor | object | `{"interval":"30s","labels":{},"metricRelabelings":[],"relabelings":[],"scrapeTimeout":"10s"}` | configuration for the Prometheus service monitor. |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` | Node selector for scheduling pods. Allows you to specify node labels for pod assignment. |
| readinessProbe | object | `{"initialDelaySeconds":5,"periodSeconds":10,"port":8081}` | Configuration for the readiness probe. |
| readinessProbe.initialDelaySeconds | int | `5` | The initial delay before the readiness probe is initiated. |
| readinessProbe.periodSeconds | int | `10` | The frequency (in seconds) with which the probe will be performed. |
| readinessProbe.port | int | `8081` | The port for the readiness check endpoint. |
| replicaCount | int | `1` | The number of replicas for the deployment. |
| tolerations | list | `[]` | Node tolerations for scheduling pods. Allows the pods to be scheduled on nodes with matching taints. |
| volumes | list | `[{"name":"cert","secret":{"defaultMode":420,"secretName":"webhook-server-cert"}}]` | Configuration for the volumes used in the deployment. |
| webhookService | object | `{"ports":{"port":443,"protocol":"TCP","targetPort":9443},"type":"ClusterIP"}` | Configuration for the webhook service. |

