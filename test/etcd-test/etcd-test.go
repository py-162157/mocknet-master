package main

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	config := clientv3.Config{
		Endpoints:   []string{"192.168.122.100:22379"},
		DialTimeout: 10 * time.Second,
	}
	client, err := clientv3.New(config)
	if err != nil {
		panic(err)
	} else {
		fmt.Println("successfully create new etcd client!")
	}
	defer client.Close()

	/*kv := clientv3.NewKV(client)

	ctx, _ := context.WithTimeout(context.TODO(), 5*time.Second)

	getResp, err := kv.Get(ctx, "/vnf-agent", clientv3.WithPrefix())
	//_, err = kv.Put(ctx, "/vnf-agent/mocknet-pod-h1/config/vpp/v2/interfaces/memif3", "{\"name\":\"memif3\",\"type\":\"MEMIF\",\"enabled\":true,\"memif\":{\"id\":1,\"socket_filename\":\"/run/vpp/memif.sock\"}}")
	if err != nil {
		panic(err)
	}
	for _, kv := range getResp.Kvs {
		fmt.Println("key:", string(kv.Key))
		fmt.Println("value:", string(kv.Value))
	}*/

	kvs := clientv3.NewKV(client)
	/*_, err = kvs.Delete(context.Background(), "/", clientv3.WithPrefix())
	if err != nil {
		panic("error when clear all key-value")
	}
	fmt.Println("clear all info finished")*/

	/*getResp, err := kvs.Get(context.Background(), "", clientv3.WithPrefix())

	if err != nil {
		panic("error when clear all key-value")
	}
	for _, kv := range getResp.Kvs {
		fmt.Println("key:", string(kv.Key))
		fmt.Println("value:", string(kv.Value))
	}*/
	//key := "/vnf-agent/mocknet-pod-s1/config/vpp/l2/v2/bridge-domain/4"
	//value := "{\"name\":\"4\",\"interfaces\":[{\"name\":\"memif0/1\"}, {\"name\":\"memif0/2\"}, {\"name\":\"tap0\"}]}"\

	key := "/vnf-agent/mocknet-pod-h1/config/vpp/l2/v2/xconnect/tap"
	value := "{\"transmit_interface\":\"memif0/\", \"receive_interface\":\"tap\"}"

	/*key := "/vnf-agent/mocknet-pod-h1/config/vpp/v2/route/if/tap0/vrf/0/dst/10.1.2.47/32/gw/10.1.2.47"
	value := "{\"dst_network\":\"10.1.2.47/32\", \"next_hop_addr\":\"10.1.2.47\", \"outgoing_interface\": \"tap0\"}"*/
	_, err = kvs.Put(context.Background(), key, value)
	if err != nil {
		panic(err)
	}

	/*interfaces := Interfaces{
		Ints: []BridgeDomain_Interface{
			{
				Name: "memif1/0",
			},
			{
				Name: "memif2/0",
			},
		},
	}

	json_result, _ := json.Marshal(interfaces)
	fmt.Println(string(json_result))*/
}

type BridgeDomain_Interface struct {
	Name                    string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	BridgedVirtualInterface bool     `protobuf:"varint,2,opt,name=bridged_virtual_interface,json=bridgedVirtualInterface,proto3" json:"bridged_virtual_interface,omitempty"`
	SplitHorizonGroup       uint32   `protobuf:"varint,3,opt,name=split_horizon_group,json=splitHorizonGroup,proto3" json:"split_horizon_group,omitempty"`
	XXX_NoUnkeyedLiteral    struct{} `json:"-"`
	XXX_unrecognized        []byte   `json:"-"`
	XXX_sizecache           int32    `json:"-"`
}

type Interfaces struct {
	Ints []BridgeDomain_Interface
}
