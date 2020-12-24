// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package scaffold

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gruntwork-io/terratest/modules/k8s"
	corev1 "k8s.io/api/core/v1"
)

var (
	_apisixConfigMap = `
kind: ConfigMap
apiVersion: v1
metadata:
  name: apisix-gw-config.yaml
data:
  config-default.yaml: |
%s
  config.yaml: |
%s
`
	_apisixDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: apisix-deployment-e2e-test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: apisix-deployment-e2e-test
  strategy:
    rollingUpdate:
      maxSurge: 50%
      maxUnavailable: 1
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: apisix-deployment-e2e-test
    spec:
      terminationGracePeriodSeconds: 0
      containers:
        - livenessProbe:
            failureThreshold: 3
            initialDelaySeconds: 2
            periodSeconds: 5
            successThreshold: 1
            tcpSocket:
              port: 9080
            timeoutSeconds: 2
          readinessProbe:
            failureThreshold: 3
            initialDelaySeconds: 2
            periodSeconds: 5
            successThreshold: 1
            tcpSocket:
              port: 9080
            timeoutSeconds: 2
          image: "apache/apisix:latest"
          imagePullPolicy: IfNotPresent
          name: apisix-deployment-e2e-test
          ports:
            - containerPort: 9080
              name: "http"
              protocol: "TCP"
            - containerPort: 9180
              name: "http-admin"
              protocol: "TCP"
          volumeMounts:
            - mountPath: /usr/local/apisix/conf/config.yaml
              name: apisix-config-yaml-configmap
              subPath: config.yaml
            - mountPath: /usr/local/apisix/conf/config-default.yaml
              name: apisix-config-yaml-configmap
              subPath: config-default.yaml
      volumes:
        - configMap:
            name: apisix-gw-config.yaml
          name: apisix-config-yaml-configmap
`
	_apisixService = `
apiVersion: v1
kind: Service
metadata:
  name: apisix-service-e2e-test
spec:
  selector:
    app: apisix-deployment-e2e-test
  ports:
    - name: http
      port: 9080
      protocol: TCP
      targetPort: 9080
    - name: http-admin
      port: 9180
      protocol: TCP
      targetPort: 9180
  type: NodePort
`
)

func (s *Scaffold) apisixServiceURL() (string, error) {
	if len(s.nodes) == 0 {
		return "", errors.New("no available node")
	}
	for _, port := range s.apisixService.Spec.Ports {
		if port.Name == "http" {
			// Basically we use minikube, so just use the first node.
			return fmt.Sprintf("http://%s:%d", s.nodes[0], port.NodePort), nil
		}
	}
	return "", errors.New("no http port in apisix service")
}

func (s *Scaffold) newAPISIX() (*corev1.Service, error) {
	defaultData, err := s.renderConfig(s.opts.APISIXDefaultConfigPath)
	if err != nil {
		return nil, err
	}
	data, err := s.renderConfig(s.opts.APISIXConfigPath)
	if err != nil {
		return nil, err
	}
	defaultData = indent(defaultData)
	data = indent(data)
	configData := fmt.Sprintf(_apisixConfigMap, defaultData, data)
	if err := k8s.KubectlApplyFromStringE(s.t, s.kubectlOptions, configData); err != nil {
		return nil, err
	}
	if err := k8s.KubectlApplyFromStringE(s.t, s.kubectlOptions, _apisixDeployment); err != nil {
		return nil, err
	}
	if err := k8s.KubectlApplyFromStringE(s.t, s.kubectlOptions, _apisixService); err != nil {
		return nil, err
	}

	svc, err := k8s.GetServiceE(s.t, s.kubectlOptions, "apisix-service-e2e-test")
	if err != nil {
		return nil, err
	}
	return svc, nil
}

func indent(data string) string {
	list := strings.Split(data, "\n")
	for i := 0; i < len(list); i++ {
		list[i] = "    " + list[i]
	}
	return strings.Join(list, "\n")
}