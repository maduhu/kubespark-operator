/*
Copyright 2016 Iguazio Systems Ltd.

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
package main

import (
	"time"
	"github.com/zmhassan/sparkcluster-crd/client"
	"github.com/zmhassan/sparkcluster-crd/crd"
	apiextcs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/kubernetes"
	apiv1 "k8s.io/api/core/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/api/core/v1"
	"flag"
	"log"
	"os"
	//"io"
)

// return rest config, if path not specified assume in cluster config
func GetClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

func main() {

	kubeconf := flag.String("kubeconf", os.Getenv("HOME")+"/.kube/config", "Path to a kube config. Only required if out-of-cluster.")
	flag.Parse()

	config, err := GetClientConfig(*kubeconf)
	if err != nil {
		panic(err.Error())
	}

	// create clientset and create our CRD, this only need to run once
	clientset, err := apiextcs.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// note: if the CRD exist our CreateCRD function is set to exit without an error
	err = crd.CreateCRD(clientset)
	if err != nil {
		log.Println("Msg: CRD Already Exists")
		panic(err)
	}

	// Wait for the CRD to be created before we use it (only needed if its a new one)
	time.Sleep(3 * time.Second)

	// Create a new clientset which include our CRD schema
	crdcs, scheme, err := crd.NewClient(config)
	if err != nil {
		panic(err)
	}

	CreateCluster(config,nil)
	// Create a CRD client interface
	crdclient := client.CrdClient(crdcs, scheme, "default")


	// Watch for changes in Spark objects and fire Add, Delete, Update callbacks
	_, controller := cache.NewInformer(
		crdclient.NewListWatch(),
		&crd.SparkCluster{},
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				log.Printf("add: %s \n", obj)
				cls:=obj.(*crd.SparkCluster)

				//CreateCluster()

				//CreateCluster(config, cls)

				//clusters.CreateCluster("sparkit", "default", "radanalyticsio/openshift-spark:2.2-latest",
				//	&config,  nil, config, "sparkit", false)

				log.Println("Image is: ", cls.Spec.Image)
				log.Println("Workers is: ", cls.Spec.Workers)
				log.Println("SparkMetricsON is: ", cls.Spec.SparkMetricsOn)
				//TODO: Create logic here that will create the necessary pods that would compose of a spark cluster.

			},
			DeleteFunc: func(obj interface{}) {
				log.Printf("delete: %s \n", obj)

			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				log.Printf("Update old: %s \n      New: %s\n", oldObj, newObj)
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)

	// Wait forever
	select {}
}
func CreateCluster(config *rest.Config, sparkConfig *crd.SparkCluster) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	log.Println("~~~~~~~~~~~~~~~~~~~")
	log.Println("Creating SparkCluster")
	//Deploy Spark Master
	CreateNewSparkMaster(clientset)
	CreateNewSparkWorkers(clientset)

}

// TODO: Pass in a clusterConfig which will contain properties

type ClusterConfig struct {
	MasterSvcURI string
	ImageName string
	PodName string
	ContainerName string
	ScaleNum int32
	Labels map[string]string
	EnvVar []apiv1.EnvVar
	Ports []apiv1.ContainerPort
}


func CreateNewSparkWorkers( clientset *kubernetes.Clientset) {
	deploymentsClient := clientset.AppsV1beta1().Deployments("myproject")

	clusterCfg:=ClusterConfig{
		"sparkle-master-service",
		"radanalyticsio/openshift-spark",
		"spark-worker",
		"spark-worker",
		3,
		map[string]string{
			"app": "spark-worker",
		},[]apiv1.EnvVar{
			apiv1.EnvVar{
				Name:  "SPARK_MASTER_ADDRESS",
				Value: "spark://sparkle-master-service:7077",
			},
			apiv1.EnvVar{
				Name:  "SPARK_METRICS_ON",
				Value: "prometheus",
			},
			apiv1.EnvVar{
				Name:  "SPARK_MASTER_UI_ADDRESS",
				Value: "http://sparkle-master-service:8080",
	}},nil}

	deployment := CreatePod(clusterCfg)

	log.Println("Running Deployment..")
	result, err := deploymentsClient.Create(deployment)
	if err != nil {
		panic(err)
	}
	log.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
}

// Generic Function for pod creations
func CreatePod(config ClusterConfig) *appsv1beta1.Deployment {
	deployment := &appsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.PodName,
		},
		Spec: appsv1beta1.DeploymentSpec{
			Replicas: int32Ptr(config.ScaleNum),
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: config.Labels,
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  config.ContainerName,
							Image: config.ImageName,
							Env:   config.EnvVar,
						},
					},
				},
			},
		},
	}
	return deployment
}



func CreateNewSparkMaster( clientset *kubernetes.Clientset ) {
	deploymentsClient := clientset.AppsV1beta1().Deployments("myproject")



	clusterCfg:=ClusterConfig{
		"sparkle-master-service",
		"radanalyticsio/openshift-spark",
		"spark-master",
		"spark-master",
		1,
		map[string]string{
			"app": "spark",
		},[]apiv1.EnvVar{
			apiv1.EnvVar{
				Name:  "SPARK_MASTER_PORT",
				Value: "7077",
			},
			apiv1.EnvVar{
				Name:  "SPARK_MASTER_WEBUI_PORT",
				Value: "8080",
			},
			apiv1.EnvVar{
				Name:  "SPARK_METRICS_ON",
				Value: "prometheus",
			}},[]apiv1.ContainerPort{
			{
				Name:          "sparksubmit",
				Protocol:      apiv1.ProtocolTCP,
				ContainerPort: 7077,
			},
			{
				Name:          "prometheus",
				Protocol:      apiv1.ProtocolTCP,
				ContainerPort: 7777,
			},
			{
				Name:          "http",
				Protocol:      apiv1.ProtocolTCP,
				ContainerPort: 8080,
			},
		}}
	log.Println("Running Deployment..")
	deployment := CreatePod(clusterCfg)
	result, err := deploymentsClient.Create(deployment)
	if err != nil {
		panic(err)
	}
	log.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
	CreateSparkClusterService(clusterCfg, clientset)
}
func CreateSparkClusterService(clusterCfg ClusterConfig, clientset *kubernetes.Clientset )   {

	service:= &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterCfg.MasterSvcURI,
			Labels: map[string]string{
				"app": "spark",
			},
			OwnerReferences: []metav1.OwnerReference{},
		},
		Spec: v1.ServiceSpec{
			Type:      "ClusterIP",
			ClusterIP: "None",
			Selector: map[string]string{
				"app": "spark",
			},
			Ports: []v1.ServicePort{{
				Name: "sparksubmit",
				Port: 7077,
			}, {
				Name: "prometheus",
				Port: 7777,
			}, {
				Name: "http",
				Port: 8080,
			}},
		},
	}

	svc_result, svc_err := clientset.CoreV1().Services("myproject").Create(service)
	if svc_err != nil {
		panic(svc_err)
	}
	log.Printf("Created Service %q.\n", svc_result.GetObjectMeta().GetName())

}

func int32Ptr(i int32) *int32 { return &i }

