package namespaceDB

//
//import (
//	corev1 "k8s.io/api/core/v1"
//	"reflect"
//	"sigs.k8s.io/controller-runtime/pkg/client"
//	"sigs.k8s.io/controller-runtime/pkg/client/fake"
//	"sigs.k8s.io/controller-runtime/pkg/event"
//	"sync"
//	"testing"
//)
//
//func TestInitDB(t *testing.T) {
//	type args struct {
//		client client.Client
//	}
//	var objs []client.Object
//	objs = append(objs, GetNode(ResourceMap{"cpu": "5100m", "ram": "5G"}, "master01", "master"))
//	objs = append(objs, GetNode(ResourceMap{"cpu": "5200m", "ram": "6G"}, "master02", "master"))
//	objs2 := make([]client.Object, len(objs))
//	copy(objs2, objs)
//	objs2 = append(objs2, GetNode(ResourceMap{"cpu": "5", "ram": "5Gi"}, "worker01", "worker"))
//	objs2 = append(objs2, GetNode(ResourceMap{"cpu": "5100m", "ram": "5G"}, "worker02", "worker"))
//	tests := []struct {
//		name    string
//		args    args
//		want    *ResourceDB
//		wantErr bool
//	}{
//		{
//			name: "Contains one Group",
//			args: args{fake.NewClientBuilder().WithObjects(objs...).Build()},
//			want: NewResourceDBTemplate().AddGroup("master").
//				AddNode("master", "master01", ResourceMap{"cpu": "5100m", "ram": "5G"}).
//				AddNode("master", "master02", ResourceMap{"cpu": "5200m", "ram": "6G"}).ResourceDB,
//			wantErr: false,
//		},
//		{
//			name: "Contains two Groups",
//			args: args{fake.NewClientBuilder().WithObjects(objs2...).Build()},
//			want: NewResourceDBTemplate().AddGroup("master").
//				AddGroup("worker").
//				AddNode("master", "master01", ResourceMap{"cpu": "5100m", "ram": "5G"}).
//				AddNode("master", "master02", ResourceMap{"cpu": "5200m", "ram": "6G"}).
//				AddNode("worker", "worker01", ResourceMap{"cpu": "5", "ram": "5Gi"}).
//				AddNode("worker", "worker02", ResourceMap{"cpu": "5100m", "ram": "5G"}).ResourceDB,
//			wantErr: false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			got, err := InitDB(tt.args.client)
//			if got != nil {
//				got.mutex = nil
//				got.NodeControllerEvents = nil
//				tt.want.mutex = nil
//				tt.want.NodeControllerEvents = nil
//			}
//			if (err != nil) != tt.wantErr {
//				t.Errorf("InitDB() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("InitDB() = \n %v \n want \n %v", got, tt.want)
//			}
//		})
//	}
//}
//
//func TestResourceDB_GetAllocatableResource(t *testing.T) {
//	type fields struct {
//		groupResources map[string]NodeResources
//		mutex          *sync.RWMutex
//	}
//	type args struct {
//		nodeGroup string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//		want   corev1.ResourceList
//	}{
//		{
//			name: "3 worker sum",
//			fields: fields{NewResourceDBTemplate().AddGroup("master").
//				AddGroup("worker").
//				AddNode("master", "master01", ResourceMap{"cpu": "5100m", "ram": "5G"}).
//				AddNode("master", "master02", ResourceMap{"cpu": "5200m", "ram": "6G"}).
//				AddNode("worker", "worker01", ResourceMap{"cpu": "100m", "ram": "2Ki"}).
//				AddNode("worker", "worker02", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).
//				AddNode("worker", "worker03", ResourceMap{"cpu": "200m", "ram": "4Ki"}).ResourceDB.groupResources,
//				&sync.RWMutex{}},
//			args: args{"worker"},
//			want: GetResourceList(map[corev1.ResourceName]string{"cpu": "5400m", "ram": "9Ki"}),
//		},
//	}
//	//resource.Quantity{{10100, -3},{nil}, "10100m", "DecimalSI"}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			rdb := &ResourceDB{
//				groupResources: tt.fields.groupResources,
//				mutex:          tt.fields.mutex,
//			}
//			got, _ := rdb.GetAllocatableResource(tt.args.nodeGroup)
//			for k, _ := range got {
//				gotVal := got[k]
//				wantVal := tt.want[k]
//				if gotVal.Value() != wantVal.Value() {
//					t.Errorf("GetAllocatableResource() = \n %v \n want \n %v", got, tt.want)
//				}
//			}
//		})
//	}
//}
//
//func TestResourceDB_CreateGroup(t *testing.T) {
//	type fields struct {
//		groupResources map[string]NodeResources
//		mutex          *sync.RWMutex
//	}
//	type args struct {
//		group string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{{
//		name:   "map initialize",
//		fields: fields{NewResourceDBTemplate().ResourceDB.groupResources, &sync.RWMutex{}},
//		args:   args{"worker"},
//	}}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			rdb := &ResourceDB{
//				groupResources: tt.fields.groupResources,
//				mutex:          tt.fields.mutex,
//			}
//			rdb.CreateGroup(tt.args.group)
//			if m := rdb.groupResources[tt.args.group]; m == nil {
//				t.Errorf("map for group %v isnt initialized", tt.args.group)
//			}
//		})
//	}
//}
//
//func TestResourceDB_DelNodeResources(t *testing.T) {
//	type fields struct {
//		groupResources       map[string]NodeResources
//		mutex                *sync.RWMutex
//		NodeControllerEvents chan event.GenericEvent
//	}
//	type args struct {
//		nodeGroup string
//		nodeName  string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{
//		{
//			name: "last in group",
//			fields: fields{NewResourceDBTemplate().
//				AddGroup("worker").
//				AddNode("worker", "worker02", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).groupResources,
//				&sync.RWMutex{},
//				make(chan event.GenericEvent)},
//			args: args{
//				nodeGroup: "worker",
//				nodeName:  "worker02",
//			},
//		},
//		{
//			name: "not last in group",
//			fields: fields{NewResourceDBTemplate().
//				AddGroup("worker").
//				AddNode("worker", "worker02", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).
//				AddNode("worker", "worker01", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).groupResources,
//				&sync.RWMutex{},
//				make(chan event.GenericEvent)},
//			args: args{
//				nodeGroup: "worker",
//				nodeName:  "worker01",
//			},
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			rdb := &ResourceDB{
//				groupResources:       tt.fields.groupResources,
//				mutex:                tt.fields.mutex,
//				NodeControllerEvents: tt.fields.NodeControllerEvents,
//			}
//			rdb.DelNodeResources(tt.args.nodeGroup, tt.args.nodeName)
//			if m, ok := rdb.groupResources[tt.args.nodeGroup]; ok {
//				for node, _ := range m {
//					if node == tt.args.nodeName {
//						t.Errorf("node %v didnt delete", tt.args.nodeName)
//					}
//				}
//			}
//		})
//	}
//}
//
//func TestResourceDB_GetGroupNodes(t *testing.T) {
//	type fields struct {
//		groupResources       map[string]NodeResources
//		mutex                *sync.RWMutex
//		NodeControllerEvents chan event.GenericEvent
//	}
//	type args struct {
//		group string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//		want   []string
//	}{
//		{
//			name: "non zero",
//			fields: fields{NewResourceDBTemplate().
//				AddGroup("worker").
//				AddNode("worker", "worker02", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).
//				AddNode("worker", "worker01", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).groupResources,
//				&sync.RWMutex{},
//				make(chan event.GenericEvent)},
//			args: args{
//				group: "worker",
//			},
//			want: StringSliceSort([]string{"worker02", "worker01"}),
//		},
//		{
//			name: "zero",
//			fields: fields{NewResourceDBTemplate().groupResources,
//				&sync.RWMutex{},
//				make(chan event.GenericEvent)},
//			args: args{
//				group: "worker",
//			},
//			want: nil,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			rdb := &ResourceDB{
//				groupResources:       tt.fields.groupResources,
//				mutex:                tt.fields.mutex,
//				NodeControllerEvents: tt.fields.NodeControllerEvents,
//			}
//			if got := rdb.GetGroupNodes(tt.args.group); !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("GetGroupNodes() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
//
//func TestResourceDB_GetGroupResources(t *testing.T) {
//	type fields struct {
//		groupResources       map[string]NodeResources
//		mutex                *sync.RWMutex
//		NodeControllerEvents chan event.GenericEvent
//	}
//	type args struct {
//		nodeGroup string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//		want   NodeResources
//		want1  bool
//	}{
//		{
//			name: "non existing group",
//			fields: fields{NewResourceDBTemplate().groupResources,
//				&sync.RWMutex{},
//				make(chan event.GenericEvent)},
//			args: args{
//				nodeGroup: "worker",
//			},
//			want:  nil,
//			want1: false,
//		},
//		{
//			name: "existing group",
//			fields: fields{NewResourceDBTemplate().
//				AddGroup("worker").
//				AddNode("worker", "worker02", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).groupResources,
//				&sync.RWMutex{},
//				make(chan event.GenericEvent)},
//			args: args{
//				nodeGroup: "worker",
//			},
//			want:  NodeResources{"worker02": GetResourceList(ResourceMap{"cpu": "5100m", "ram": "3Ki"})},
//			want1: true,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			rdb := &ResourceDB{
//				groupResources:       tt.fields.groupResources,
//				mutex:                tt.fields.mutex,
//				NodeControllerEvents: tt.fields.NodeControllerEvents,
//			}
//			got, got1 := rdb.GetGroupResources(tt.args.nodeGroup)
//			if got1 != tt.want1 {
//				t.Errorf("GetGroupResources() got1 = %v, want %v", got1, tt.want1)
//			}
//			if got1 {
//				if !reflect.DeepEqual(got, tt.want) {
//					t.Errorf("GetGroupResources() got = %v, want %v", got, tt.want)
//				}
//			}
//		})
//	}
//}
//
//func TestResourceDB_GetNodeGroup(t *testing.T) {
//	type fields struct {
//		groupResources       map[string]NodeResources
//		mutex                *sync.RWMutex
//		NodeControllerEvents chan event.GenericEvent
//	}
//	type args struct {
//		nodeName string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//		want   string
//		want1  bool
//	}{
//		{
//			name: "node exists",
//			fields: fields{NewResourceDBTemplate().
//				AddGroup("worker").
//				AddGroup("master").
//				AddNode("worker", "worker01", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).
//				AddNode("worker", "worker02", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).
//				AddNode("master", "master01", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).
//				AddNode("master", "master02", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).groupResources,
//				&sync.RWMutex{},
//				make(chan event.GenericEvent)},
//			args: args{
//				nodeName: "worker01",
//			},
//			want:  "worker",
//			want1: true,
//		},
//		{
//			name: "node doesnt exist",
//			fields: fields{NewResourceDBTemplate().
//				AddGroup("worker").
//				AddGroup("master").
//				AddNode("worker", "worker01", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).
//				AddNode("worker", "worker02", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).
//				AddNode("master", "master01", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).
//				AddNode("master", "master02", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).groupResources,
//				&sync.RWMutex{},
//				make(chan event.GenericEvent)},
//			args: args{
//				nodeName: "worker03",
//			},
//			want:  "",
//			want1: false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			rdb := &ResourceDB{
//				groupResources:       tt.fields.groupResources,
//				mutex:                tt.fields.mutex,
//				NodeControllerEvents: tt.fields.NodeControllerEvents,
//			}
//			got, got1 := rdb.GetNodeGroup(tt.args.nodeName)
//			if got != tt.want {
//				t.Errorf("GetNodeGroup() got = %v, want %v", got, tt.want)
//			}
//			if got1 != tt.want1 {
//				t.Errorf("GetNodeGroup() got1 = %v, want %v", got1, tt.want1)
//			}
//		})
//	}
//}
//
//func TestResourceDB_GetNodeResources(t *testing.T) {
//	type fields struct {
//		groupResources       map[string]NodeResources
//		mutex                *sync.RWMutex
//		NodeControllerEvents chan event.GenericEvent
//	}
//	type args struct {
//		nodeGroup string
//		nodeName  string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//		want   corev1.ResourceList
//	}{
//		{
//			name: "node exists",
//			fields: fields{NewResourceDBTemplate().
//				AddGroup("worker").
//				AddNode("worker", "worker01", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).groupResources,
//				&sync.RWMutex{},
//				make(chan event.GenericEvent)},
//			args: args{
//				nodeGroup: "worker",
//				nodeName:  "worker01",
//			},
//			want: GetResourceList(ResourceMap{"cpu": "5100m", "ram": "3Ki"}),
//		},
//		{
//			name: "node doesnt exist",
//			fields: fields{NewResourceDBTemplate().
//				AddGroup("worker").
//				AddNode("worker", "worker01", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).groupResources,
//				&sync.RWMutex{},
//				make(chan event.GenericEvent)},
//			args: args{
//				nodeGroup: "worker",
//				nodeName:  "worker02",
//			},
//			want: nil,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			rdb := &ResourceDB{
//				groupResources:       tt.fields.groupResources,
//				mutex:                tt.fields.mutex,
//				NodeControllerEvents: tt.fields.NodeControllerEvents,
//			}
//			if got := rdb.GetNodeResources(tt.args.nodeGroup, tt.args.nodeName); !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("GetNodeResources() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
//
//func TestResourceDB_SetNodeResources(t *testing.T) {
//	type fields struct {
//		groupResources       map[string]NodeResources
//		mutex                *sync.RWMutex
//		NodeControllerEvents chan event.GenericEvent
//	}
//	type args struct {
//		nodeGroup string
//		nodeName  string
//		r         corev1.ResourceList
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{
//		{
//			name: "node exists",
//			fields: fields{NewResourceDBTemplate().
//				AddGroup("worker").
//				AddNode("worker", "worker01", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).groupResources,
//				&sync.RWMutex{},
//				make(chan event.GenericEvent)},
//			args: args{
//				nodeGroup: "worker",
//				nodeName:  "worker01",
//				r:         GetResourceList(ResourceMap{"cpu": "5200m", "ram": "4Ki"}),
//			},
//		},
//		{
//			name: "node doesnt exist",
//			fields: fields{NewResourceDBTemplate().
//				AddGroup("worker").
//				AddNode("worker", "worker01", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).groupResources,
//				&sync.RWMutex{},
//				make(chan event.GenericEvent)},
//			args: args{
//				nodeGroup: "worker",
//				nodeName:  "worker02",
//				r:         GetResourceList(ResourceMap{"cpu": "5200m", "ram": "4Ki"}),
//			},
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			rdb := &ResourceDB{
//				groupResources:       tt.fields.groupResources,
//				mutex:                tt.fields.mutex,
//				NodeControllerEvents: tt.fields.NodeControllerEvents,
//			}
//			rdb.SetNodeResources(tt.args.nodeGroup, tt.args.nodeName, tt.args.r)
//			if !reflect.DeepEqual(rdb.groupResources[tt.args.nodeGroup][tt.args.nodeName], tt.args.r) {
//				t.Errorf("SetNodeResources() = %v, want %v", rdb.groupResources[tt.args.nodeGroup][tt.args.nodeName], tt.args.r)
//			}
//		})
//	}
//}
//
//func TestResourceDB_notifyNodeGroup(t *testing.T) {
//	type fields struct {
//		groupResources       map[string]NodeResources
//		mutex                *sync.RWMutex
//		NodeControllerEvents chan event.GenericEvent
//	}
//	type args struct {
//		group string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{
//		{
//			name: "send event to channel",
//			fields: fields{NewResourceDBTemplate().
//				AddGroup("worker").
//				AddNode("worker", "worker01", ResourceMap{"cpu": "5100m", "ram": "3Ki"}).groupResources,
//				&sync.RWMutex{},
//				make(chan event.GenericEvent)},
//			args: args{
//				group: "worker",
//			},
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			rdb := &ResourceDB{
//				groupResources:       tt.fields.groupResources,
//				mutex:                tt.fields.mutex,
//				NodeControllerEvents: tt.fields.NodeControllerEvents,
//			}
//			go rdb.notifyNodeGroup(tt.args.group)
//			obj := <-rdb.NodeControllerEvents
//			if obj.Object.GetName() != tt.args.group {
//				t.Errorf("notifyNodeGroup() = node group is different than object name: got %v, want %v", obj.Object.GetName(), tt.args.group)
//			}
//		})
//	}
//}
