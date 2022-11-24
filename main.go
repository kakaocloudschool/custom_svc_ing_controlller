package main

import (
	"flag"
	"fmt"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// 컨피그 파일 불러옴 ( ~/.kube/config 는 안되서 다른 걸로 함. )
	kubeconfig := flag.String("kubeconfig", "/root/.kube/config", "location to your kubeconfig file")
	// clientcmd -> 쿠버네티스 접근을 위한 접속 정보
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	// handle error
	if err != nil {
		// 클러스터 내부에서 접근해서 데이터를 가져오는 방법
		fmt.Printf("error %s", err)
		config, err = rest.InClusterConfig()
		if err != nil {
			fmt.Printf("InClusterConfig error %s", err)
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("error %s", err)
	}

	ch := make(chan struct{})
	informers := informers.NewSharedInformerFactory(clientset, 10*time.Minute)
	c := newController(clientset, informers.Apps().V1().Deployments())
	informers.Start(ch)
	c.run(ch)

	fmt.Println(informers)
}
