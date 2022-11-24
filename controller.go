package main

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type controller struct {
	clientset      kubernetes.Interface
	depLister      appslisters.DeploymentLister
	depCacheSynced cache.InformerSynced
	queue          workqueue.RateLimitingInterface
}

func newController(clientset kubernetes.Interface, depInformer appsinformers.DeploymentInformer) *controller {
	c := &controller{
		clientset:      clientset,
		depLister:      depInformer.Lister(),
		depCacheSynced: depInformer.Informer().HasSynced,
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ekspose"),
	}

	depInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handleAdd,
			DeleteFunc: c.handleDel,
		},
	)

	return c
}

func (c *controller) run(ch <-chan struct{}) {
	fmt.Println("startung contoller")
	// 현재 서버의 캐시로 현재 캐시를 초기화 함. 아래 조건은 실패할 경우의 작업을 나타냄.
	if !cache.WaitForCacheSync(ch, c.depCacheSynced) {
		fmt.Print("wating for cache to be synced\n")
	}

	// 통신을 위한 채널이 종료되지 않는다면, 1초마다 c.worker 라는 행위를 반복함
	go wait.Until(c.worker, 1*time.Second, ch)

	// chanel 의 응답 대기 용도, 위의 고 루틴은 채널이 끝나면 종료되기에, 있을때 까지 대기하기 위해 사용
	<-ch
}

func (c *controller) worker() {
	for c.proccessItem() {

	}
}

func (c *controller) proccessItem() bool {
	// 현재 큐에서 데이터가 있는 지 조회한다.
	item, shutdown := c.queue.Get()
	// 데이터가 없는 경우 아래와 같이 루프를 종료하기 위해 return 한다.
	if shutdown {
		return false
	}
	defer c.queue.Forget(item)
	// 데이터가 있는 경우 item의 cache 데이터에 정보를 불러온다.
	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		fmt.Print("getting key from cache %s\n", err.Error())
	}

	// key 정보를 바탕으로 네임 스페이스를 분리한다.
	// 현재는 에러를 별도로 처리하고 있지는 않음.
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Print("SplitMetaNamespaceKey error %s \n", err.Error())
		return false
	}

	// 원하는 작업을 수행한다.
	err = c.syncDeployment(ns, name)
	if err != nil {
		// 실패 하더라도, 재 시도는 기본적으로 수행함으로 retry 를 할 필요가 있다.
		fmt.Print("syncing deployment %s\n", err.Error())
		return false
	}
	return true
}

func (c *controller) syncDeployment(ns, name string) error {
	// context 선언 : context 란 고 루틴상의 작업 명세서의 역할을 수행한다.
	// 해당 작업 명세에 따라, 취소되고 처리하고 등의 작업을 수행한다.
	ctx := context.Background()

	//우리는 서비스를 생성해야 함으로,
	// 서비스의 대상이 되는 디플로이먼트 명을 알기 위해, deployment 명을 조회한다.
	dep, err := c.depLister.Deployments(ns).Get(name)

	if err != nil {
		fmt.Printf("getting deployment from lister %s", err.Error())
	}

	//create service
	// 서비스의 경우 아래에서 생성하는건 80 포트로 고정함으로, 수정할 필요가 있다면 수정해야 한다.
	// 현재 만드는 작업은 svc 를 만들기 위한 메타 정보를 작성하는 것이다.
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dep.Name,
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Selector: depLabels(*dep),
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
			},
		},
	}
	// 서비스를 생성한다.
	_, err = c.clientset.CoreV1().Services(ns).Create(ctx, &svc, metav1.CreateOptions{})

	if err != nil {
		fmt.Printf("creating error %s", err.Error())
	}

	//create ingress

	return nil
}

func depLabels(dep appsv1.Deployment) map[string]string {
	return dep.Spec.Template.Labels
}

func (c *controller) handleAdd(obj interface{}) {
	fmt.Println("add was called")
	c.queue.Add(obj)

}

func (c *controller) handleDel(obj interface{}) {
	fmt.Println("del was called")
	c.queue.Add(obj)
}
