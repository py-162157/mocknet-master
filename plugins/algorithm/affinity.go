package affinity

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"time"

	"log"
	"mocknet/plugins/server/rpctest"
)

type element Node
type PartitionResult []uint

type ArrayUnion struct {
	group map[element]element
	size  map[element]uint
	items map[element][]element
}

type Edge struct {
	start  element
	end    element
	weight uint
}

type Affinity struct {
	k               uint
	E               []Edge
	V               []element
	uf              ArrayUnion
	clost_neighbors map[element]element
	merged          map[element]element
}

func (af *Affinity) new_and_init(edges []Edge, k uint) {
	v_set := make(map[element]element)
	for _, e := range edges {
		v_set[e.start] = e.start
		v_set[e.end] = e.end
	}
	v := make([]element, 0)
	for key, _ := range v_set {
		v = append(v, key)
	}
	log.Println("number of vertexs is:", len(v))
	af.k = k
	af.E = edges
	af.V = v
	af.uf = Au_new_and_init(v)
	af.clost_neighbors = make(map[element]element, 0)
	af.merged = make(map[element]element, 0)
}

func (af Affinity) print_all_clusters() {
	for name, clusters := range af.uf.items {
		log.Println("cluster = ", name, clusters)
		log.Println()
	}
}

func (af *Affinity) merge_with_cloest_neighbors(v element, v_stack map[element]element) map[element]element {
	if _, ok := af.merged[v]; ok {
		return v_stack
	}
	if self_closet, ok := af.clost_neighbors[v]; ok {
		if self_cloest_cloest, ok := af.clost_neighbors[self_closet]; ok {
			if v == self_cloest_cloest {
				findv := af.uf.find(v)
				find_clost_neighbor := af.uf.find(self_closet)
				tag := af.uf.try_union(findv, find_clost_neighbor)
				if tag == true {
					af.merged[v] = v
					af.merged[self_closet] = self_closet
				}
				return v_stack
			} else {
				findv := af.uf.find(v)
				find_clost_neighbor := af.uf.find(self_closet)
				if _, ok := v_stack[v]; ok {
					return v_stack
				} else {
					v_stack[v] = v
					v_stack = af.merge_with_cloest_neighbors(self_closet, v_stack)
					delete(v_stack, v)
					tag := af.uf.try_union(findv, find_clost_neighbor)
					if tag == true {
						af.merged[v] = v
						af.merged[af.clost_neighbors[v]] = v
					}
					return v_stack
				}
			}
		}
	}
	return v_stack
}

func (af *Affinity) fragment_process(round uint) {
	init_group := af.uf.get_items()
	for _, group_name := range init_group {
		if _, ok := af.uf.items[group_name]; ok {
			if af.uf.size[group_name] < uint(math.Pow(2, float64(round))) {
				edges_of_froup := make([]Edge, 0)
				for _, e := range af.E {
					if e.start == group_name {
						edges_of_froup = append(edges_of_froup, e)
					}
				}
				sort.Slice(edges_of_froup, func(i, j int) bool {
					return edges_of_froup[i].weight <= edges_of_froup[j].weight
				})
				for _, e := range edges_of_froup {
					if af.uf.find(e.end) != group_name {
						af.uf.try_union(group_name, e.end)
					}
				}
			}
		}
	}
}

func (af *Affinity) edges_update() {
	new_edges := make([]Edge, 0)
	for _, e := range af.E {
		if af.uf.find(e.start) != af.uf.find(e.end) {
			new_edges = append(new_edges, Edge{
				start:  af.uf.find(e.start),
				end:    af.uf.find(e.end),
				weight: e.weight,
			})
		}
	}
	af.E = new_edges
}

func (af *Affinity) clustering(FragmentProcess bool, CommonNeighborCluster bool) {
	number_of_clusters := len(af.V)
	vertexs := af.V
	count := 0
	for {
		if number_of_clusters <= int(af.k) || count >= 5 {
			break
		}
		count += 1
		log.Println("-----------------------------------after ", count, " rounds clustering-------------------------------")
		af.clost_neighbors = make(map[element]element)
		af.merged = make(map[element]element)
		selfe := af.E
		EEV := edges_of_every_vertexs(selfe)
		min_edges := make([]Edge, 0)
		v_stack := make(map[element]element)

		for _, edges := range EEV {
			sort.Slice(edges, func(i, j int) bool {
				return edges[i].weight <= edges[j].weight
			})
			if CommonNeighborCluster == true {
				for i, j := 0, len(edges)-1; i < j; i, j = i+1, j-1 {
					edges[i], edges[j] = edges[j], edges[i]
				}
			}
			min_edges = append(min_edges, edges[0])
		}

		for _, edge := range min_edges {
			af.clost_neighbors[edge.start] = edge.end
		}

		for _, v := range vertexs {
			if _, ok := af.merged[v]; !ok {
				v_stack = af.merge_with_cloest_neighbors(v, v_stack)
			}
		}

		af.edges_update()

		vertexs = af.uf.get_items()
		number_of_clusters = len(vertexs)

		if FragmentProcess == true {
			af.fragment_process(uint(count))
			af.edges_update()
		}
		log.Println("present number of clusters is:", number_of_clusters)
	}
}

func edges_of_every_vertexs(edges []Edge) map[element][]Edge {
	edges_of_v := make(map[element][]Edge)
	for _, edge := range edges {
		if _, ok := edges_of_v[edge.start]; ok {
			edges_of_v[edge.start] = append(edges_of_v[edge.start], edge)
		} else {
			edges_of_v[edge.start] = []Edge{edge}
		}

		if _, ok := edges_of_v[edge.end]; ok {
			edges_of_v[edge.end] = append(edges_of_v[edge.end], edge)
		} else {
			edges_of_v[edge.end] = []Edge{edge}
		}
	}
	return edges_of_v
}

func (af Affinity) linear_embed() []element {
	line := make([]element, 0)
	for _, cluster := range af.uf.items {
		line = append(line, cluster...)
	}
	return line
}

// ---------------------------ArrayUnion------------------------------
func Au_new_and_init(V []element) ArrayUnion {
	ArrUni := ArrayUnion{
		group: make(map[element]element),
		size:  make(map[element]uint),
		items: make(map[element][]element),
	}
	for _, v := range V {
		ArrUni.group[v] = v
		ArrUni.size[v] = 1
		ArrUni.items[v] = []element{v}
	}
	return ArrUni
}

// 返回包含target的group id
func (au ArrayUnion) find(target element) element {
	if gp, ok := au.group[target]; ok {
		return gp
	} else {
		panic("Error happened when finding the target's group!")
	}
}

func (au *ArrayUnion) try_union(a element, b element) bool {
	ag, ok1 := au.items[a]
	bg, ok2 := au.items[b]
	if !(ok1 && ok2) {
		//log.Println("a and b are not both in items")
		return false
	}

	if is_equal(ag, bg) {
		log.Println("These two group are in a same items")
		return false
	}

	if au.size[a] > au.size[b] {
		temp := a
		a = b
		b = temp
	}

	for _, s := range au.items[a] {
		au.group[a] = b
		au.items[b] = append(au.items[b], s)
	}

	au.size[b] += au.size[a]
	delete(au.size, a)
	delete(au.items, a)
	return true
}

func (au ArrayUnion) get_items() []element {
	keys := make([]element, 0)
	for key, _ := range au.items {
		keys = append(keys, key)
	}
	return keys
}

func is_equal(A []element, B []element) bool {
	if len(A) != len(B) {
		return false
	}
	for i, _ := range A {
		if A[i] != B[i] {
			return false
		}
	}
	return true
}

type EdgeSlice []Edge

func (a EdgeSlice) len() int           { return len(a) }
func (a EdgeSlice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a EdgeSlice) Less(i, j int) bool { return a[i].weight <= a[j].weight }

func MST(edges []Edge) []Edge {
	mst := make([]Edge, 0)
	sort.Slice(edges, func(i, j int) bool {
		return edges[i].weight <= edges[j].weight
	})
	v_set := make(map[element]element)
	for _, e := range edges {
		v_set[e.start] = e.start
		v_set[e.end] = e.end
	}
	V := make([]element, 0)
	for key, _ := range v_set {
		V = append(V, key)
	}
	UF := Au_new_and_init(V)
	for _, e := range edges {
		u_group := UF.find(e.start)
		v_group := UF.find(e.end)

		if u_group != v_group {
			mst = append(mst, e)
			UF.try_union(u_group, v_group)
		}
	}
	return mst
}

type Partition struct {
	ele element
	id  PartitionKey
}

type PartitionKey struct {
	key  uint
	edge Edge
}

func partition1(v_edges map[element][]Edge, k uint) []Partition {
	out := make([]Partition, 0)
	partition_key := rand.Intn(int(k))
	for _, edges := range v_edges {
		for _, e := range edges {
			out = append(out, Partition{
				ele: e.end,
				id: PartitionKey{
					key:  uint(partition_key),
					edge: e,
				},
			})
		}
	}
	return out
}

func group_by_end(edges []Partition) map[element][]PartitionKey {
	out := make(map[element][]PartitionKey)
	for _, edge := range edges {
		out[edge.ele] = append(out[edge.ele], PartitionKey{
			key:  edge.id.key,
			edge: edge.id.edge,
		})
	}
	return out
}

func print_edges(edges []Edge) {
	for _, edge := range edges {
		log.Println("start:", edge.start, ", end:", edge.end, ", weight:", edge.weight)
	}
}

func make_cluster(edges []Edge, threshold uint, FragmentProcess bool, CommonNeighborCluster bool) Affinity {
	v_set := make(map[element]element)
	for _, e := range edges {
		v_set[e.start] = e.start
		v_set[e.end] = e.end
	}
	af := Affinity{}
	af.new_and_init(edges, threshold)
	af.clustering(FragmentProcess, CommonNeighborCluster)
	return af
}

type Node struct {
	name   string
	weight uint
}

type HashEdge struct {
	start element
	end   element
}

func get_hash_edges(edges []Edge) map[HashEdge]uint {
	hash_edges := make(map[HashEdge]uint)
	for _, edge := range edges {
		hash_edges[HashEdge{
			start: element(edge.start),
			end:   element(edge.end),
		}] = edge.weight
	}
	return hash_edges
}

func find_commen_neighbors(edges []Edge) []Edge {
	neighbors_of_vertex := make(map[element][]element)
	edges_of_common_neighbors := make(map[HashEdge]uint)
	new_edges := make([]Edge, 0)
	for _, e := range edges {
		neighbors_of_vertex[e.start] = append(neighbors_of_vertex[e.start], e.end)
	}
	for _, neighbors := range neighbors_of_vertex {
		edge_weight := 1
		for _, vertex1 := range neighbors {
			for _, vertex2 := range neighbors {
				if vertex1 != vertex2 {
					if _, ok := edges_of_common_neighbors[HashEdge{start: element(vertex1), end: element(vertex2)}]; ok {
						edges_of_common_neighbors[HashEdge{start: element(vertex1), end: element(vertex2)}] += uint(edge_weight)
					} else {
						edges_of_common_neighbors[HashEdge{start: element(vertex1), end: element(vertex2)}] = uint(edge_weight)
					}
				}
			}
		}
	}

	for pair, weight := range edges_of_common_neighbors {
		new_edges = append(new_edges, Edge{
			start:  element(pair.start),
			end:    element(pair.end),
			weight: weight,
		})
	}

	return new_edges
}

func makerange(min, max int) []uint {
	a := make([]uint, max-min)
	for i := range a {
		a[i] = uint(min) + uint(i)
	}
	return a
}

func RankSwap(line []element, k uint, r uint, q []uint) []element {
	log.Println("begin to rank swap, k=", k, "r=", r, "q=", q)
	divided_line := make([][][]element, 0)
	cut_size := make([]uint, 0)
	for _, i := range makerange(0, len(q)-1) {
		partition_size := 0
		partition := make([][]element, 0)
		interval_index := make([]uint, 0)
		for _, j := range makerange(0, int(r+1)) {
			interval_index = append(interval_index,
				q[i]+uint(math.Floor(float64((j*(q[i+1]-q[i])))/float64(r))),
			)
		}
		for _, j := range makerange(0, int(r)) {
			interval := make([]element, 0)
			for _, k := range makerange(int(interval_index[j]), int(interval_index[j+1])) {
				interval = append(interval, line[k])
				partition_size += int(line[k].weight)
			}
			sort.Slice(interval, func(i2, j2 int) bool {
				return interval[i2].weight >= interval[j2].weight
			})
			partition = append(partition, interval)
		}
		divided_line = append(divided_line, partition)
		cut_size = append(cut_size, uint(partition_size))
	}
	log.Println("Begin to optimize the cut size")

	hash_pair := make(map[uint]uint)
	for _, j := range makerange(0, int(r)) {
		for {
			random_pair := rand.Intn(int(r))
			if _, ok := hash_pair[uint(random_pair)]; !ok {
				hash_pair[uint(random_pair)] = j
				break
			}
		}
	}
	log.Println("randomly pair intervals completed!")

	cut_size_copy := cut_size
	partition_size_rank := make([]uint, 0)
	for _, _ = range makerange(0, int(k)) {
		max := 0
		max_position := 0
		for _, j := range makerange(0, int(k)) {
			if int(cut_size_copy[j]) > max {
				max = int(cut_size_copy[j])
				max_position = int(j)
			}
		}
		partition_size_rank = append(partition_size_rank, uint(max_position))
		cut_size_copy[max_position] = 0
	}
	partition_pairs := make(map[uint]uint)
	for _, i := range makerange(0, int(k/2)) {
		partition_pairs[partition_size_rank[i]] = partition_size_rank[k-i-1]
	}

	count := 0
	for {
		count += 1
		log.Println("After ", count, " rounds swap ")
		flag := 0
		for partition1, partition2 := range partition_pairs {
			for interval1, interval2 := range hash_pair {
				for _, j := range makerange(0, len(divided_line[partition1][interval1])) {
					best_pair := -1
					present_small_max_size := math.Max(float64(cut_size[partition1]), float64(cut_size[partition2]))
					for _, k := range makerange(0, len(divided_line[partition2][interval2])) {
						imaginaty_max_size := math.Max(
							float64(cut_size[partition1]-divided_line[partition1][interval1][j].weight+divided_line[partition2][interval2][k].weight),
							float64(cut_size[partition2]-divided_line[partition2][interval2][k].weight+divided_line[partition1][interval1][j].weight),
						)
						if imaginaty_max_size < present_small_max_size {
							best_pair = int(k)
							present_small_max_size = imaginaty_max_size
						}
					}
					if best_pair != -1 {
						real_best_pair := best_pair
						flag = 1
						pre_weight := divided_line[partition2][interval2][real_best_pair].weight
						post_weight := divided_line[partition1][interval1][j].weight
						difference := uint(math.Abs(float64(pre_weight) - float64(post_weight)))
						if pre_weight > post_weight {
							cut_size[partition1] += difference
							cut_size[partition2] -= difference
						} else {
							cut_size[partition2] += difference
							cut_size[partition1] -= difference
						}
						temp := divided_line[partition1][interval1][j]
						divided_line[partition1][interval1][j] = divided_line[partition2][interval2][real_best_pair]
						divided_line[partition2][interval2][real_best_pair] = temp
					}

				}
			}
		}
		if flag == 0 {
			break
		}
	}

	adjusted_line := make([]element, 0)
	for _, partition := range divided_line {
		for _, interval := range partition {
			adjusted_line = append(adjusted_line, interval...)
		}
	}
	return adjusted_line
}

func q_list(k uint) []uint {
	hash_qs := make(map[uint]uint)
	present_k1 := k
	present_k2 := k
	hash_qs[k] = k
	for {
		if present_k1 == 1 || present_k2 == 1 {
			break
		}
		hash_number := make(map[uint]uint)
		hash_number[present_k1/2] = 1
		hash_number[present_k1-present_k1/2] = 1
		hash_number[present_k2/2] = 1
		hash_number[present_k2-present_k2/2] = 1
		if len(hash_number) == 1 {
			for key, _ := range hash_number {
				present_k1 = key
			}
			present_k2 = present_k1
			hash_qs[present_k1] = 1
		} else {
			count := 1
			for key, _ := range hash_number {
				if count == 1 {
					present_k1 = key
				} else {
					present_k2 = key
				}
				count += 1
			}
			hash_qs[present_k1] = 1
			hash_qs[present_k2] = 1
		}
	}
	qs := make([]uint, 0)
	for key, _ := range hash_qs {
		qs = append(qs, key)
	}
	sort.Slice(qs, func(i, j int) bool {
		return qs[i] <= qs[j]
	})
	log.Println("The q_list is:", qs)
	return qs
}

func Array2(line1, line2 uint) [][]uint {
	array := make([][]uint, 0)
	for _, _ = range makerange(0, int(line1)) {
		array = append(array, make([]uint, line2))
	}
	return array
}

func Array2Float(line1, line2 uint) [][]float64 {
	array := make([][]float64, 0)
	for _, _ = range makerange(0, int(line1)) {
		array = append(array, make([]float64, line2))
	}
	return array
}

func Array3(line1, line2, line3 uint) [][][]uint {
	array := make([][][]uint, 0)
	for _, _ = range makerange(0, int(line1)) {
		array1 := make([][]uint, 0)
		for _, _ = range makerange(0, int(line2)) {
			array1 = append(array1, make([]uint, line3))
		}
		array = append(array, array1)
	}
	return array
}

type cost struct {
	edges_weight_cut_off uint
	max_cluster_weight   uint
	min_cluster_weight   uint
	variance             float64
	cluster_weight_mean  float64
	cut_points           PartitionResult
}

func Array3Cost(line1, line2, line3 uint) [][][]cost {
	array := make([][][]cost, 0)
	for _, _ = range makerange(0, int(line1)) {
		array1 := make([][]cost, 0)
		for _, _ = range makerange(0, int(line2)) {
			array1 = append(array1, make([]cost, line3))
		}
		array = append(array, array1)
	}
	return array
}

func DynamicProgram(line []element, k uint, edges map[HashEdge]uint, alpha float64, origin_edges []Edge) map[string]uint {
	if alpha < 0 || alpha > 1 {
		panic("The weight coefficient must be [0, 1]!")
	}
	fmt.Println("             ")
	fmt.Println("             ")
	fmt.Println("             ")
	fmt.Println("alpha =", alpha)
	node_position := make(map[element]uint)
	for _, i := range makerange(0, len(line)) {
		node_position[line[i]] = i
	}

	vertex_num := len(line)
	total_node_weight := 0
	total_edge_weight := 0

	// Table J：三维矩阵(𝑣𝑛 × 𝑣𝑛 ×𝑣𝑛) ，𝐽(𝑖,𝑗,𝑘) 存储了一端位于区间(𝑖,𝑗)中的点，另一端连接至点k的所有边权总和，其计算公式为：
	// J(i, j, k) = J(i, j-1, k) + J(j, j, k)
	J := Array3(uint(vertex_num), uint(vertex_num), uint(vertex_num))
	for _, i := range makerange(0, vertex_num) {
		total_node_weight += int(line[i].weight)
	}
	for _, edge := range origin_edges {
		start := edge.start
		end := edge.end
		total_edge_weight += int(edge.weight)
		// 当i, j之间有edge时, 初始化J(i, i, j)为weight
		J[node_position[start]][node_position[start]][node_position[end]] = edge.weight
	}
	for _, length := range makerange(2, vertex_num) {
		for _, i := range makerange(0, vertex_num-int(length)) {
			j := i + length - 1
			for _, k := range makerange(int(i+length), vertex_num) {
				J[i][j][k] = J[i][j-1][k] + J[j][j][k]
			}
		}
	}
	log.Println("table J has been completed")

	// 二维矩阵(𝑣𝑛 × 𝑣𝑛) ，D(𝑖,𝑗) 存储了两端均在区间(𝑖,𝑗)的所有边权总和，其计算公式为:
	// D(i, j+1) = D(i, j) + J(i, j, j+1)
	/*D := Array2(uint(vertex_num), uint(vertex_num))
	for _, i := range makerange(0, vertex_num-1) {
		for _, j := range makerange(int(i+1), vertex_num) {
			D[i][j] = D[i][j-1] + J[i][j-1][j]
		}
	}*/
	log.Println("table D has been completed")

	// 二维矩阵(𝑣𝑛 × 𝑣𝑛) ，B(𝑖,𝑗) 存储了区间(𝑖,𝑗)内的所有点权总和，其计算公式为：
	// B(i, j+1) = B(i, j) + W(j+1)
	B := Array2(uint(vertex_num), uint(vertex_num))
	for _, i := range makerange(0, vertex_num) {
		B[i][i] = line[i].weight
	}
	for _, i := range makerange(0, vertex_num-1) {
		for _, j := range makerange(int(i+1), vertex_num) {
			B[i][j] = B[i][j-1] + line[j].weight
		}
	}
	log.Println("table B has been completed")

	// 二维矩阵E, 存储了从(i, j)区间的均值
	E := Array2Float(uint(vertex_num), uint(vertex_num))
	for _, i := range makerange(0, vertex_num) {
		E[i][i] = float64(line[i].weight)
	}
	for _, i := range makerange(0, vertex_num-1) {
		for _, j := range makerange(int(i+1), vertex_num) {
			E[i][j] = (E[i][j-1]*float64(j-i) + float64(line[j].weight)) / float64(j-i+1)
		}
	}
	log.Println("table E has been completed")

	// 二维矩阵V, 存储了从(i, j)区间的方差
	V := Array2Float(uint(vertex_num), uint(vertex_num))
	for _, i := range makerange(0, vertex_num-1) {
		for _, j := range makerange(int(i+1), vertex_num) {
			V[i][j] = ((V[i][j-1]+math.Pow(E[i][j-1], 2))*float64(j-1)+float64(line[j].weight))/float64(j-i+1) - math.Pow(E[i][j], 2)
		}
	}
	log.Println("table V has been completed")

	// 三维矩阵(𝑣𝑛 × 𝑣𝑛 ×𝑣𝑛) ，C(𝑖, 𝑗, k) 存储了一端位于区间(𝑖,𝑘)，另一端位于区间(𝑘,𝑗)内的所有边权总和，其计算公式为：
	// C(i, j, k) = C(i, j-1, k) + J(i, k, j)
	C := Array3(uint(vertex_num), uint(vertex_num), uint(vertex_num))
	for _, i := range makerange(0, vertex_num-1) {
		C[i][i+1][i] = J[i][i][i+1]
	}
	for _, i := range makerange(0, vertex_num-2) {
		for _, j := range makerange(int(i+2), vertex_num) {
			for _, cut_point := range makerange(int(i), int(j)) {
				C[i][j][cut_point] = C[i][j-1][cut_point] + J[i][cut_point][j]
			}
		}
	}
	log.Println("table C has been completed")

	// A(i, j, k)存储了将子问题[i, j]分割为k部分的最优解的切断边权总和，最大cluster和最小cluster
	// Ap(i, j, k)存储了将子问题[i, j]分割为k部分的最优解的切割位置

	// type cost struct {
	// 	   edges_weight_cut_off uint
	// 	   max_cluster_weight   uint
	// 	   min_cluster_weight   uint
	// }
	A := Array3Cost(uint(vertex_num), uint(vertex_num), k+1)
	log.Println("The average weight of cluster is:", float32(total_node_weight+total_edge_weight)/float32(k))

	/*for _, i := range makerange(0, vertex_num) {
		for _, j := range makerange(0, vertex_num) {
			for _, k := range makerange(0, vertex_num) {
				fmt.Println("J(", i, ",", j, ",", k, ") =", J[i][j][k])
			}
		}
	}

	for _, i := range makerange(0, vertex_num) {
		for _, j := range makerange(0, vertex_num) {
			for _, k := range makerange(0, vertex_num) {
				fmt.Println("C(", i, ",", j, ",", k, ") =", C[i][j][k])
			}
		}
	}*/

	for _, i := range makerange(0, vertex_num) {
		for _, j := range makerange(int(i), vertex_num) {
			//A[i][j][1] = B[i][j] + D[i][j] + C[0][j][i] + C[i][vertex_num-1][j]
			A[i][j][1] = cost{
				edges_weight_cut_off: 0,
				max_cluster_weight:   B[i][j],
				min_cluster_weight:   B[i][j],
				variance:             0,
				cluster_weight_mean:  float64(B[i][j]),

				//max_cluster_weight:   B[i][j] + D[i][j],
				//min_cluster_weight:   B[i][j] + D[i][j],
			}
		}
		A[i][i][1].cut_points = []uint{}
	}
	log.Println("dynamic programing started")

	max_edge_weight := uint(0)
	for _, weight := range edges {
		if weight > uint(max_edge_weight) {
			max_edge_weight = weight
		}
	}

	qs := q_list(k)
	qs = qs[1:]
	for _, q := range qs {
		println("now running in q = ", q)
		left := q / 2
		right := q - left
		for _, i := range makerange(0, vertex_num-1) {
			for _, j := range makerange(int(i+1), vertex_num) {
				if q <= j-i+1 {
					min_cut_point := i
					min_cost := float64(100)

					added_edges_weight := uint(0)
					for _, cut_point := range makerange(int(i), int(j)) {
						if cut_point-i+1 >= left && j-cut_point >= right {
							Aleft := A[i][cut_point][left]
							Aright := A[cut_point+1][j][right]
							// A(i, j, q) = Min{ A(i, k, q/2) , A(k+1, j, q-q/2) }

							// 1. 在该点切割所产生的最大cluster = max(左子问题的最大cluster, 右子问题的最大cluster)
							//max_cluster_weight := math.Max(float64(Aleft.max_cluster_weight), float64(Aright.max_cluster_weight))
							// 2. 在该点切割所产生的最小cluster = min(左子问题的最小cluster, 右子问题的最小cluster)
							//min_cluster_weight := math.Min(float64(Aleft.min_cluster_weight), float64(Aright.min_cluster_weight))

							left_cluster_num := float64((len(Aleft.cut_points) + 1))
							right_cluster_num := float64((len(Aright.cut_points) + 1))
							present_cluster_weight_mean := (Aleft.cluster_weight_mean*left_cluster_num + Aright.cluster_weight_mean*right_cluster_num) / (left_cluster_num + right_cluster_num)

							/*// 被切一刀后新增的并不是C[i][j][cut_point]
							left_border := uint(0)
							right_border := uint(0)
							if left == 1 {
								left_border = i
							} else {
								left_border = A[i][cut_point][left].cut_points[len(A[i][cut_point][left].cut_points)-1]
							}

							if right == 1 {
								right_border = j
							} else {
								right_border = A[cut_point+1][j][right].cut_points[0]
							}*/

							// 3. 在该点切割所切断的edge weight总和 = 左子问题的edge weight + 右子问题的edge weight + 新产生的cost
							edge_weight_cut_off := Aleft.edges_weight_cut_off + Aright.edges_weight_cut_off + C[i][j][cut_point]
							M := math.Sqrt(float64(edge_weight_cut_off) / float64((k-1)*uint(vertex_num)*max_edge_weight))
							// 左右子问题总方差 = 组内方差 + 组间方差
							total_variance := (float64(len(Aleft.cut_points)+1)*(Aleft.variance+math.Pow(Aleft.cluster_weight_mean, 2))+float64(len(Aright.cut_points)+1)*(Aright.variance+math.Pow(Aright.cluster_weight_mean, 2)))/(left_cluster_num+right_cluster_num) - math.Pow(present_cluster_weight_mean, 2)
							N := math.Sqrt(math.Sqrt(total_variance) / present_cluster_weight_mean)
							current_cost := alpha*M + (1-alpha)*N
							if current_cost < min_cost {
								min_cost = current_cost
								min_cut_point = cut_point
								added_edges_weight = C[i][j][cut_point]
							}

							/*if i == 0 && j == uint(vertex_num)-1 && q == k {
								fmt.Println("alpha =", alpha, ", M =", M, ", N =", N, ", COST =", current_cost)
							}*/
						}
					}
					Aleft := A[i][min_cut_point][left]
					Aright := A[min_cut_point+1][j][right]
					A[i][j][q].edges_weight_cut_off = Aleft.edges_weight_cut_off + Aright.edges_weight_cut_off + added_edges_weight
					A[i][j][q].max_cluster_weight = uint(math.Max(float64(Aleft.max_cluster_weight), float64(Aright.max_cluster_weight)))
					A[i][j][q].min_cluster_weight = uint(math.Min(float64(Aleft.min_cluster_weight), float64(Aright.min_cluster_weight)))
					left_cluster_num := float64((len(Aleft.cut_points) + 1))
					right_cluster_num := float64((len(Aright.cut_points) + 1))
					A[i][j][q].cluster_weight_mean = (Aleft.cluster_weight_mean*left_cluster_num + Aright.cluster_weight_mean*right_cluster_num) / (left_cluster_num + right_cluster_num)
					A[i][j][q].variance = (float64(len(Aleft.cut_points)+1)*(Aleft.variance+math.Pow(Aleft.cluster_weight_mean, 2))+float64(len(Aright.cut_points)+1)*(Aright.variance+math.Pow(Aright.cluster_weight_mean, 2)))/(left_cluster_num+right_cluster_num) - math.Pow(A[i][j][q].cluster_weight_mean, 2)
					A[i][j][q].cut_points = A[i][min_cut_point][left].cut_points
					A[i][j][q].cut_points = append(A[i][j][q].cut_points, min_cut_point)
					right_part := A[min_cut_point+1][j][right].cut_points
					A[i][j][q].cut_points = append(A[i][j][q].cut_points, right_part...)
					if i == 0 && j == uint(vertex_num)-1 && q == k {
						log.Println("debug! The C is: ", C[i][j][min_cut_point], "cost is: ", added_edges_weight)
					}
				}
			}
		}
	}
	log.Println("dynamic programing finished!")
	log.Println("These cut point is: ", A[0][vertex_num-1][k].cut_points)
	log.Println("The A is: ", A[0][vertex_num-1][k])

	cluster_size := make([]uint, 0)
	cut_index := make([]uint, 0)
	cut_index = append(cut_index, 0)
	cut_index = append(cut_index, A[0][vertex_num-1][k].cut_points...)
	cut_index = append(cut_index, uint(vertex_num)-1)
	log.Println("The cut_index is: ", cut_index)

	worker_index := 1
	worker_assign := make(map[string]uint)
	for _, i := range makerange(0, len(cut_index)-1) {
		present_cluster_size := uint(0)
		if i == 0 {
			for _, j := range makerange(int(cut_index[i]), int(cut_index[i+1]+1)) {
				worker_assign[line[j].name] = uint(worker_index)
			}
		} else {
			for _, j := range makerange(int(cut_index[i]+1), int(cut_index[i+1]+1)) {
				worker_assign[line[j].name] = uint(worker_index)
			}
		}

		worker_index += 1

		/*if i == 0 {
			present_cluster_size += C[0][vertex_num-1][cut_index[1]]
		} else if i == uint(len(cut_index)-2) {
			present_cluster_size += C[0][vertex_num-1][cut_index[i]]
		} else {
			present_cluster_size += C[0][cut_index[i+1]][cut_index[i]]
			present_cluster_size += C[cut_index[i]][vertex_num-1][cut_index[i+1]]
		}*/
		//present_cluster_size += D[cut_index[i]][cut_index[i+1]]
		if i == 0 {
			present_cluster_size += B[0][cut_index[1]]
		} else {
			present_cluster_size += B[cut_index[i]+1][cut_index[i+1]]
		}
		if present_cluster_size != 0 {
			cluster_size = append(cluster_size, present_cluster_size)
		}
	}

	edge_weight_cut_off := 0
	for _, edge := range origin_edges {
		if worker_assign[edge.start.name] != worker_assign[edge.end.name] {
			edge_weight_cut_off += int(edge.weight)
			//fmt.Println("start: ", worker_assign[edge.start.name], "end:", worker_assign[edge.end.name], "weight =", edge.weight)
		}
	}

	mean := float64(0)
	variance := float64(0)
	for _, i := range makerange(0, len(cluster_size)) {
		mean += float64(cluster_size[i]) / float64(len(cluster_size))
	}
	for _, i := range makerange(0, len(cluster_size)) {
		variance += math.Pow(float64(cluster_size[i])-mean, 2)
	}
	variance /= float64(len(cluster_size))
	standard_deviation := math.Sqrt(variance)
	coefficient_of_variation := standard_deviation / mean
	max_size := A[0][vertex_num-1][k].max_cluster_weight
	min_size := A[0][vertex_num-1][k].min_cluster_weight
	log.Println("The sizes is:", cluster_size)
	log.Println("The mean size is:", mean)
	log.Println("The max size is:", max_size)
	log.Println("The min size is:", min_size)
	log.Println("These edges cut off are:", A[0][vertex_num-1][k].edges_weight_cut_off)
	log.Println("These edges cut off new are:", edge_weight_cut_off/2)
	log.Println("The coefficient of variance is:", coefficient_of_variation)
	log.Println("The coefficient of variance new is:", math.Sqrt(A[0][vertex_num-1][k].variance)/mean)
	fmt.Println("             ")
	fmt.Println("             ")
	fmt.Println("             ")
	//N := float64(max_size-min_size) / float64(max_size)
	return worker_assign
	//return edge_weight_cut_off / 2, N
}

func Combination(af Affinity, partition_number uint, interval_len uint, rank_swap bool) []element {
	q := make([]uint, 0)
	line := af.linear_embed()
	log.Println(line)
	for _, i := range makerange(0, int(partition_number)+1) {
		q = append(q, uint(math.Floor(float64(i*uint(len(line)))/float64(partition_number))))
	}
	if rank_swap {
		adjusted_line := RankSwap(line, partition_number, interval_len, q)
		return adjusted_line
	} else {
		return line
	}
}

func make_random_graph(vertex uint) []Edge {
	i := 0
	j := 0
	edges := make([]Edge, 0)
	random_weight := make([]uint, 0)
	for _, _ = range makerange(0, int(vertex)) {
		r_w := rand.Intn(5) + 1
		random_weight = append(random_weight, uint(r_w))
	}
	for {
		if i >= int(vertex) {
			break
		}
		j = 0
		for {
			if j >= int(vertex) {
				break
			}
			if i == j {
				j += 1
			} else if j < i {
				j += 1
			} else {
				rand_number1 := rand.Intn(j-i) + 1
				if rand_number1 != 1 {

				} else {
					edges = append(edges, Edge{
						start: element(Node{
							name:   strconv.Itoa(i),
							weight: random_weight[i],
						}),
						end: element(Node{
							name:   strconv.Itoa(j),
							weight: random_weight[j],
						}),
						weight: 1,
					})
					edges = append(edges, Edge{
						start: element(Node{
							name:   strconv.Itoa(j),
							weight: random_weight[j],
						}),
						end: element(Node{
							name:   strconv.Itoa(i),
							weight: random_weight[i],
						}),
						weight: 1,
					})
				}
				j += 1
			}
		}
		i += 1
	}
	return edges
}

/*func Random_mock(scale uint, partition_number uint, rank_swap bool, threshold uint) {
	edges := make_random_graph(scale)
	hash_edges := get_hash_edges(edges)
	new_edges := find_commen_neighbors(edges)
	af := make_cluster(new_edges, threshold, true, true)
	line_after_swap := Combination(af, partition_number, uint(math.Sqrt(float64(scale/partition_number))), rank_swap)

	DynamicProgram(line_after_swap, partition_number, hash_edges)
}*/

func generate_edges(message rpctest.Message) ([]Edge, uint) {
	topo := message.Command.EmunetCreation.Emunet
	edges := make([]Edge, 0)

	node_weights := make(map[string]uint)
	for _, edge := range topo.Links {
		node_weights[edge.Node1.Name] += 1
	}

	for _, edge := range topo.Links {
		edges = append(edges, Edge{
			start: element(Node{
				name:   edge.Node1.Name,
				weight: node_weights[edge.Node1.Name],
			}),
			end: element(Node{
				name:   edge.Node2.Name,
				weight: node_weights[edge.Node2.Name],
			}),
		})
	}
	return edges, uint(len(topo.Pods))
}

func RandSlice(slice interface{}) { //切片乱序
	rv := reflect.ValueOf(slice)
	if rv.Type().Kind() != reflect.Slice {
		return
	}

	length := rv.Len()
	if length < 2 {
		return
	}

	swap := reflect.Swapper(slice)
	rand.Seed(time.Now().Unix())
	for i := length - 1; i >= 0; i-- {
		j := rand.Intn(length)
		swap(i, j)
	}
	return
}

func Random_partition(edges []Edge, line []element, partition_number uint, hash_edges map[HashEdge]uint) (int, float64) {
	RandSlice(line)

	q := make([]uint, 0)
	//log.Println(line)
	for _, i := range makerange(0, int(partition_number)+1) {
		q = append(q, uint(math.Floor(float64(i*uint(len(line)))/float64(partition_number))))
	}
	if q[len(q)-1] > uint(len(line)-1) {
		q[len(q)-1] = uint(len(line) - 1)
	}
	//fmt.Println("The mean cut point is: ", q)

	node_position := make(map[element]uint)
	for _, i := range makerange(0, len(line)) {
		node_position[line[i]] = i
	}

	vertex_num := len(line)
	total_node_weight := 0
	total_edge_weight := 0

	J := Array3(uint(vertex_num), uint(vertex_num), uint(vertex_num))
	for _, i := range makerange(0, vertex_num) {
		total_node_weight += int(line[i].weight)
	}
	for hashedge, weight := range hash_edges {
		start := hashedge.start
		end := hashedge.end
		total_edge_weight += int(weight)
		J[node_position[start]][node_position[start]][node_position[end]] = 1
	}
	for _, length := range makerange(2, vertex_num) {
		for _, i := range makerange(0, vertex_num-int(length)) {
			j := i + length - 1
			for _, k := range makerange(int(i+length), vertex_num) {
				J[i][j][k] = J[i][j-1][k] + J[j][j][k]
			}
		}
	}

	/*D := Array2(uint(vertex_num), uint(vertex_num))
	for _, i := range makerange(0, vertex_num-1) {
		for _, j := range makerange(int(i+1), vertex_num) {
			D[i][j] = D[i][j-1] + J[i][j-1][j]
		}
	}*/

	B := Array2(uint(vertex_num), uint(vertex_num))
	for _, i := range makerange(0, vertex_num) {
		B[i][i] = line[i].weight
	}
	for _, i := range makerange(0, vertex_num-1) {
		for _, j := range makerange(int(i+1), vertex_num) {
			B[i][j] = B[i][j-1] + line[j].weight
		}
	}

	C := Array3(uint(vertex_num), uint(vertex_num), uint(vertex_num))
	for _, i := range makerange(0, vertex_num-1) {
		C[i][i+1][i] = J[i][i][i+1]
	}
	for _, i := range makerange(0, vertex_num-2) {
		for _, j := range makerange(int(i+2), vertex_num) {
			for _, cut_point := range makerange(int(i), int(j)) {
				C[i][j][cut_point] = C[i][j-1][cut_point] + J[i][cut_point][j]
			}
		}
	}

	worker_index := 1
	worker_assign := make(map[string]uint)
	cluster_size := make([]uint, 0)
	max_cluster_size := 0
	min_cluster_size := math.MaxInt
	for _, i := range makerange(0, len(q)-1) {
		if i == 0 {
			for _, j := range makerange(int(q[i]), int(q[i+1]+1)) {
				worker_assign[line[j].name] = uint(worker_index)
			}
		} else {
			for _, j := range makerange(int(q[i]+1), int(q[i+1]+1)) {
				worker_assign[line[j].name] = uint(worker_index)
			}
		}

		worker_index += 1

		present_cluster_size := uint(0)
		if i == 0 {
			present_cluster_size += C[0][vertex_num-1][q[1]]
		} else if i == uint(len(q)-2) {
			present_cluster_size += C[0][vertex_num-1][q[i]]
		} else {
			present_cluster_size += C[0][q[i+1]][q[i]]
			present_cluster_size += C[q[i]][vertex_num-1][q[i+1]]
		}
		//present_cluster_size += D[q[i]][q[i+1]]
		present_cluster_size += B[q[i]][q[i+1]]
		cluster_size = append(cluster_size, present_cluster_size)
		if present_cluster_size > uint(max_cluster_size) {
			max_cluster_size = int(present_cluster_size)
		}

		if present_cluster_size < uint(min_cluster_size) {
			min_cluster_size = int(present_cluster_size)
		}
	}

	edge_weight_cut_off := 0
	for _, edge := range edges {
		if worker_assign[edge.start.name] != worker_assign[edge.end.name] {
			edge_weight_cut_off += int(edge.weight)
		}
	}

	mean := float64(0)
	variance := float64(0)
	for _, i := range makerange(0, len(cluster_size)) {
		mean += float64(cluster_size[i]) / float64(len(cluster_size))
	}
	for _, i := range makerange(0, len(cluster_size)) {
		variance += math.Pow(float64(cluster_size[i])-mean, 2)
	}
	variance /= float64(len(cluster_size))
	standard_deviation := math.Sqrt(variance)
	coefficient_of_variation := standard_deviation / mean
	/*log.Println("The sizes is:", cluster_size)
	log.Println("The mean size is:", mean)
	log.Println("The max size is:", max_cluster_size)
	log.Println("The min size is:", min_cluster_size)
	log.Println("These edges cut off are:", edge_weight_cut_off/2)
	log.Println("The coefficient of variance is:", coefficient_of_variation)*/
	//N := float64(max_cluster_size-min_cluster_size) / float64(max_cluster_size)

	return edge_weight_cut_off / 2, coefficient_of_variation
	//return edge_weight_cut_off / 2, N
}

func Generate_Tree_Topo(m int, n int) ([]Edge, []element, uint) {
	nodes := make([]element, 0)
	edges := make([]Edge, 0)
	cut_point := (1 - int(math.Pow(float64(n), float64(m)))) / (1 - n)      // 最后一个switch节点的序号，从1开始
	total_number := (1 - int(math.Pow(float64(n), float64(m+1)))) / (1 - n) // 总节点数
	for _, i := range makerange(1, cut_point+1) {
		nodes = append(nodes, element(Node{
			name:   "s" + strconv.Itoa(int(i)),
			weight: 3,
		}))
	}
	for _, i := range makerange(cut_point+1, total_number+1) {
		parent_index := (int(i) + n - 2) / n
		nodes = append(nodes, element(Node{
			name:   nodes[parent_index-1].name + "h" + strconv.Itoa((int(i)+n-2)%n+1),
			weight: 1,
		}))
	}
	for _, i := range makerange(2, total_number+1) {
		edges = append(edges, Edge{
			start:  element(nodes[(int(i)+n-2)/n-1]),
			end:    element(nodes[int(i)-1]),
			weight: 1,
		})
		edges = append(edges, Edge{
			start:  element(nodes[int(i)-1]),
			end:    element(nodes[(int(i)+n-2)/n-1]),
			weight: 1,
		})
	}
	return edges, nodes, uint(len(nodes))
}

type Pod struct {
	aggregations []Node
	grounds      []Ground
}

type Ground struct {
	access Node
	hosts  []Node
}

type FatTree struct {
	cores []Node
	pods  []Pod
}

func Generate_Fat_Tree_Topo(n int, random bool) ([]Edge, []element, uint) {
	edges := make([]Edge, 0)
	nodes := make([]element, 0)
	var fat_tree FatTree
	edges_hash := make(map[element]int, 0)

	for _, i := range makerange(1, n+1) {
		fat_tree.cores = append(fat_tree.cores, Node{
			name:   "core" + strconv.Itoa(int(i)),
			weight: uint(n),
		})

		var pod Pod
		for _, j := range makerange(1, n/2+1) {
			pod.aggregations = append(pod.aggregations, Node{
				name:   "aggt" + strconv.Itoa((int(i)-1)*n/2+int(j)),
				weight: uint(n),
			})

			var ground Ground
			for _, k := range makerange(1, n/2+1) {
				ground.access = Node{
					name:   "accs" + strconv.Itoa((int(i)-1)*n/2+int(j)),
					weight: uint(n),
				}

				ground.hosts = append(ground.hosts, Node{
					name:   "host" + strconv.Itoa((int(i)-1)*n*n/4+(int(j)-1)*n/2+int(k)),
					weight: 1,
				})

			}
			pod.grounds = append(pod.grounds, ground)
		}
		fat_tree.pods = append(fat_tree.pods, pod)
	}

	for i, core := range fat_tree.cores {
		aggregation_count := i * 2 / n
		for _, pod := range fat_tree.pods {
			r := rand.Intn(5) + 1
			weight := uint(1)
			if random {
				weight = uint(r)
			}
			edges = append(edges, Edge{
				start:  element(core),
				end:    element(pod.aggregations[aggregation_count]),
				weight: weight,
			})
			edges = append(edges, Edge{
				start:  element(pod.aggregations[aggregation_count]),
				end:    element(core),
				weight: weight,
			})
		}
	}

	for _, pod := range fat_tree.pods {
		for _, aggregation := range pod.aggregations {
			for _, ground := range pod.grounds {
				r := rand.Intn(5) + 1
				weight := uint(1)
				if random {
					weight = uint(r)
				}
				edges = append(edges, Edge{
					start:  element(aggregation),
					end:    element(ground.access),
					weight: weight,
				})
				edges = append(edges, Edge{
					start:  element(ground.access),
					end:    element(aggregation),
					weight: weight,
				})
			}
		}
	}

	for _, pod := range fat_tree.pods {
		for _, ground := range pod.grounds {
			for _, host := range ground.hosts {
				r := rand.Intn(5) + 1
				weight := uint(1)
				if random {
					weight = uint(r)
				}
				edges = append(edges, Edge{
					start:  element(ground.access),
					end:    element(host),
					weight: weight,
				})
				edges = append(edges, Edge{
					start:  element(host),
					end:    element(ground.access),
					weight: weight,
				})
			}
		}
	}

	for _, edge := range edges {
		edges_hash[edge.start] = 0
		edges_hash[edge.end] = 0
	}

	for node, _ := range edges_hash {
		nodes = append(nodes, node)
	}

	return edges, nodes, uint(n*n*n/4 + n*n + n)
}

func generate_test_topo() ([]Edge, []element) {
	nodes := make([]element, 0)
	for _, i := range makerange(1, 6) {
		nodes = append(nodes, element(Node{
			name:   strconv.Itoa(int(i)),
			weight: 1,
		}))
	}
	edges := []Edge{
		{
			start:  element(nodes[0]),
			end:    element(nodes[1]),
			weight: 1,
		},
		{
			start:  element(nodes[0]),
			end:    element(nodes[2]),
			weight: 1,
		},
		{
			start:  element(nodes[0]),
			end:    element(nodes[3]),
			weight: 1,
		},
		{
			start:  element(nodes[1]),
			end:    element(nodes[2]),
			weight: 1,
		},
		{
			start:  element(nodes[2]),
			end:    element(nodes[3]),
			weight: 1,
		},
		{
			start:  element(nodes[2]),
			end:    element(nodes[4]),
			weight: 1,
		},
		{
			start:  element(nodes[1]),
			end:    element(nodes[3]),
			weight: 1,
		},
		{
			start:  element(nodes[1]),
			end:    element(nodes[0]),
			weight: 1,
		},
		{
			start:  element(nodes[2]),
			end:    element(nodes[0]),
			weight: 1,
		},
		{
			start:  element(nodes[3]),
			end:    element(nodes[0]),
			weight: 1,
		},
		{
			start:  element(nodes[2]),
			end:    element(nodes[1]),
			weight: 1,
		},
		{
			start:  element(nodes[3]),
			end:    element(nodes[2]),
			weight: 1,
		},
		{
			start:  element(nodes[4]),
			end:    element(nodes[2]),
			weight: 1,
		},
		{
			start:  element(nodes[3]),
			end:    element(nodes[1]),
			weight: 1,
		},
	}

	return edges, nodes
}

func AffinityClusterPartition(message rpctest.Message, partition_number uint, alpha float64) map[string]uint {
	edges, scale := generate_edges(message)
	hash_edges := get_hash_edges(edges)
	new_edges := find_commen_neighbors(edges)
	//log.Println("new edges are:", new_edges)
	threshold := scale / partition_number
	af := make_cluster(new_edges, threshold, true, true)
	//af.print_all_clusters()
	line_after_swap := Combination(af, partition_number, uint(math.Sqrt(float64(scale/partition_number))), true)
	log.Print("line after swap is:", line_after_swap)

	return DynamicProgram(line_after_swap, partition_number, hash_edges, alpha, edges)
}
