# MockNet
一个分布式高性能网络仿真平台，在云端服务器进行一键式高性能仿真网络部署。该平台具有高并发、集群化、云规模等特性，利用云平台和容器化技术，可以在云端进行快速部署和迁移。 实验证明，仅用一台100G高性能交换机和三台40核服务器便可以仿真出bi-section带宽为70G的胖树结构网络。该项目为IEEE INFOCOM论文在投。

## 使用方法
1. 搭建好K8s集群，并在master节点上配置flannel-etcd.yaml。
2. 在本地编译安装带有高性能网卡支持的VPP版本。
3. 在master节点上运行MockNet-Master，在其余的服务器节点上运行MockNet-Worker。
4. 在用户端运行MockNet-CMD，与MockNet-Master连接后进行网络的创建工作。

## Mocknet-Master
Mocknet-Master是MockNet仿真系统的控制中心，运行在服务器集群的master机器上（即K8s的master节点）。
1. algorithm插件里集成了将网络拓扑图映射至各服务器的算法。
2. server插件与MockNet-CMD进行前后端交互。
3. etcd插件与etcd数据库进行交互，进而与集群中的各MockNet-Worker进行通信
4. kubernetes插件与K8S的API进行交互，进行网络的创建和迁移等工作。
