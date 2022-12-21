package server

import (
	danav1 "github.com/dana-team/hns/api/v1"
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type diagramServer struct {
	client client.Client
}

func NewDiagramServer(c client.Client) *diagramServer {
	return &diagramServer{client: c}
}

func (ds *diagramServer) Run() {
	http.HandleFunc("/", ds.GetDiagram)
	http.ListenAndServe(":8888", nil)
}

func (ds *diagramServer) GetDiagram(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Origin-Headers", "Content-Type")
	fmt.Fprintf(w, ds.PrintGraph(r.URL.Path[1:]))
}

func (ds *diagramServer) PrintGraph(ns string) string {
	graph := fmt.Sprintf("graph TD\n %s\n", ns)

	var snsList danav1.SubnamespaceList
	if err := ds.client.List(context.Background(), &snsList, &client.ListOptions{Namespace: ns}); err != nil {
		return ""
	}

	for _, child := range snsList.Items {
		graph += ds.PrintGraphRec(ns, child.Name)
	}

	return graph
}

func (ds *diagramServer) PrintGraphRec(parent string, ns string) string {
	graph := ""
	graph += fmt.Sprintf("%s---|%s|%s\n", parent, ds.getEdge(parent, ns), ns)

	var snsList danav1.SubnamespaceList
	if err := ds.client.List(context.Background(), &snsList, &client.ListOptions{Namespace: ns}); err != nil {
		return ""
	}

	for _, child := range snsList.Items {
		graph += ds.PrintGraphRec(ns, child.Name)

	}
	return graph
}

func (ds *diagramServer) getEdge(parant string, ns string) string {
	var subns danav1.Subnamespace
	if err := ds.client.Get(context.Background(), types.NamespacedName{Name: ns, Namespace: parant}, &subns); err != nil {
		return "-"
	}
	cpu := subns.Spec.ResourceQuotaSpec.Hard["cpu"]
	memory := subns.Spec.ResourceQuotaSpec.Hard["memory"]
	pods := subns.Spec.ResourceQuotaSpec.Hard["pods"]
	return fmt.Sprintf("<b>CPU: </b>%s<br><b>RAM: </b>%s<br><b>Pods: </b>%s", cpu.String(), memory.String(), pods.String())
}
