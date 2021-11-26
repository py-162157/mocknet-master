package affinity

import (
	"math"
	"math/rand"
	"sort"
	"strconv"

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
		log.Println("cluster{}: {:?}", name, clusters)
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
		if _, ok := out[edge.ele]; ok {
			out[edge.ele] = append(out[edge.ele], PartitionKey{
				key:  edge.id.key,
				edge: edge.id.edge,
			})
		} else {
			out[edge.ele] = []PartitionKey{PartitionKey{
				key:  edge.id.key,
				edge: edge.id.edge,
			}}
		}
	}
	return out
}

func print_edges(edges []Edge) {
	for _, edge := range edges {
		log.Println("start:", edge.start, ", end:", edge.end, ", weight:", edge.weight)
	}
}

func make_cluster(epsilon float32, edges []Edge, threshold uint, FragmentProcess bool, CommonNeighborCluster bool) Affinity {
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
	common_neighbor_reverse := make(map[element][]element)
	edges_of_common_neighbors := make(map[HashEdge]uint)
	new_edges := make([]Edge, 0)
	hash_edges := make(map[HashEdge]uint)
	for _, e := range edges {
		if _, ok := neighbors_of_vertex[e.start]; ok {
			neighbors_of_vertex[e.start] = append(neighbors_of_vertex[e.start], e.end)
		} else {
			neighbors_of_vertex[e.start] = []element{e.end}
		}
	}
	for vertex, neighbors := range neighbors_of_vertex {
		for _, neighbor := range neighbors {
			if _, ok := common_neighbor_reverse[neighbor]; ok {
				common_neighbor_reverse[neighbor] = append(common_neighbor_reverse[neighbor], vertex)
			} else {
				common_neighbor_reverse[neighbor] = []element{vertex}
			}
		}
	}
	for _, vertexs := range common_neighbor_reverse {
		for _, vertex1 := range vertexs {
			for _, vertex2 := range vertexs {
				if vertex1 != vertex2 {
					if _, ok := edges_of_common_neighbors[HashEdge{start: element(vertex1), end: element(vertex2)}]; ok {
						edges_of_common_neighbors[HashEdge{start: element(vertex1), end: element(vertex2)}] += 1
					} else {
						edges_of_common_neighbors[HashEdge{start: element(vertex1), end: element(vertex2)}] = 1
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
		hash_edges[pair] = 0
	}

	return new_edges
}

func makeRange(min, max int) []uint {
	a := make([]uint, max-min)
	for i := range a {
		a[i] = uint(min) + uint(i)
	}
	return a
}

func RankSwap(line []element, k uint, r uint, q []uint) []element {
	divided_line := make([][][]element, 0)
	cut_size := make([]uint, 0)
	for _, i := range makeRange(0, len(q)-1) {
		partition_size := 0
		partition := make([][]element, 0)
		interval_index := make([]uint, 0)
		for _, j := range makeRange(0, int(r+1)) {
			interval_index = append(interval_index,
				q[i]+uint(math.Floor(float64((j*(q[i+1]-q[i])))/float64(r))),
			)
		}
		for _, j := range makeRange(0, int(r)) {
			interval := make([]element, 0)
			for _, k := range makeRange(int(interval_index[j]), int(interval_index[j+1])) {
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
	for _, j := range makeRange(0, int(r)) {
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
	for _, _ = range makeRange(0, int(k)) {
		max := 0
		max_position := 0
		for _, j := range makeRange(0, int(k)) {
			if int(cut_size_copy[j]) > max {
				max = int(cut_size_copy[j])
				max_position = int(j)
			}
		}
		partition_size_rank = append(partition_size_rank, uint(max_position))
		cut_size_copy[max_position] = 0
	}
	partition_pairs := make(map[uint]uint)
	for _, i := range makeRange(0, int(k/2)) {
		partition_pairs[partition_size_rank[i]] = partition_size_rank[k-i-1]
	}

	count := 0
	for {
		count += 1
		log.Println("After ", count, " rounds swap ")
		flag := 0
		for partition1, partition2 := range partition_pairs {
			for interval1, interval2 := range hash_pair {
				for _, j := range makeRange(0, len(divided_line[partition1][interval1])) {
					best_pair := -1
					present_small_max_size := math.Max(float64(cut_size[partition1]), float64(cut_size[partition2]))
					for _, k := range makeRange(0, len(divided_line[partition2][interval2])) {
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
	for _, _ = range makeRange(0, int(line1)) {
		array = append(array, make([]uint, line2))
	}
	return array
}

func Array3(line1, line2, line3 uint) [][][]uint {
	array := make([][][]uint, 0)
	for _, _ = range makeRange(0, int(line1)) {
		array1 := make([][]uint, 0)
		for _, _ = range makeRange(0, int(line2)) {
			array1 = append(array1, make([]uint, line3))
		}
		array = append(array, array1)
	}
	return array
}

func ApArray3(line1, line2, line3 uint) [][][]PartitionResult {
	array := make([][][]PartitionResult, 0)
	for _, _ = range makeRange(0, int(line1)) {
		array1 := make([][]PartitionResult, 0)
		for _, _ = range makeRange(0, int(line2)) {
			array1 = append(array1, make([]PartitionResult, line3))
		}
		array = append(array, array1)
	}
	return array
}

func DynamicProgram(line []element, k uint, edges map[HashEdge]uint) map[string]uint {
	log.Println("length of line is ", len(line))
	node_position := make(map[element]uint)
	for _, i := range makeRange(0, len(line)) {
		node_position[line[i]] = i
	}

	vertex_num := len(line)
	total_node_weight := 0
	total_edge_weight := 0

	J := Array3(uint(vertex_num), uint(vertex_num), uint(vertex_num))
	for _, i := range makeRange(0, vertex_num) {
		total_node_weight += int(line[i].weight)
	}
	for hashedge, weight := range edges {
		start := hashedge.start
		end := hashedge.end
		total_edge_weight += int(weight)
		J[node_position[start]][node_position[start]][node_position[end]] = 1
	}
	for _, length := range makeRange(2, vertex_num) {
		for _, i := range makeRange(0, vertex_num-int(length)) {
			j := i + length - 1
			for _, k := range makeRange(int(i+length), vertex_num) {
				J[i][j][k] = J[i][j-1][k] + J[j][j][k]
			}
		}
	}
	log.Println("table J has been completed")

	D := Array2(uint(vertex_num), uint(vertex_num))
	for _, i := range makeRange(0, vertex_num-1) {
		for _, j := range makeRange(int(i+1), vertex_num) {
			D[i][j] = D[i][j-1] + J[i][j-1][j]
		}
	}
	log.Println("table D has been completed")

	B := Array2(uint(vertex_num), uint(vertex_num))
	for _, i := range makeRange(0, vertex_num) {
		B[i][i] = line[i].weight
	}
	for _, i := range makeRange(0, vertex_num-1) {
		for _, j := range makeRange(int(i+1), vertex_num) {
			B[i][j] = B[i][j-1] + line[j].weight
		}
	}
	log.Println("table B has been completed")

	C := Array3(uint(vertex_num), uint(vertex_num), uint(vertex_num))
	for _, i := range makeRange(0, vertex_num-1) {
		C[i][i+1][i] = J[i][i][i+1]
	}
	for _, i := range makeRange(0, vertex_num-2) {
		for _, j := range makeRange(int(i+2), vertex_num) {
			for _, cut_point := range makeRange(int(i), int(j)) {
				C[i][j][cut_point] = C[i][j-1][cut_point] + J[i][cut_point][j]
			}
		}
	}
	log.Println("table C has been completed")

	A := Array3(uint(vertex_num), uint(vertex_num), k+1)
	Ap := ApArray3(uint(vertex_num), uint(vertex_num), k+1)
	log.Println("The average weight of cluster is:", float32(total_node_weight+total_edge_weight)/float32(k))

	for _, i := range makeRange(0, vertex_num) {
		for _, j := range makeRange(int(i), vertex_num) {
			A[i][j][1] = B[i][j] + D[i][j] + C[0][j][i] + C[i][vertex_num-1][j]
		}
	}
	log.Println("dynamic programing started")
	qs := q_list(k)
	qs = qs[1:]
	for _, q := range qs {
		println("now running in q = ", q)
		left := q / 2
		right := q - left
		for _, i := range makeRange(0, vertex_num-1) {
			for _, j := range makeRange(int(i+1), vertex_num) {
				if q <= j-i+1 {
					min_cut_point := i
					min_cut_size := 10000000
					for _, cut_point := range makeRange(int(i), int(j)) {
						if cut_point-i+1 >= left && j-cut_point+1 >= right {
							cut_size_temp := int(math.Max(float64(A[i][cut_point][left]), float64(A[cut_point+1][j][right])))
							if cut_size_temp < min_cut_size {
								min_cut_size = cut_size_temp
								min_cut_point = cut_point
							}
						}
					}
					A[i][j][q] = uint(min_cut_size)
					Ap[i][j][q] = Ap[i][min_cut_point][left]
					Ap[i][j][q] = append(Ap[i][j][q], min_cut_point)
					right_part := Ap[min_cut_point+1][j][right]
					Ap[i][j][q] = append(Ap[i][j][q], right_part...)
				}
			}
		}
	}
	log.Println("dynamic programing finished!")
	log.Println("These cut point is: ", Ap[0][vertex_num-1][k])

	cluster_size := make([]uint, 0)
	cut_index := make([]uint, 0)
	cut_index = append(cut_index, 0)
	cut_index = append(cut_index, Ap[0][vertex_num-1][k]...)
	cut_index = append(cut_index, uint(vertex_num)-1)
	log.Println("The cut_index is: ", cut_index)

	max_cluster_size := 0
	worker_index := 1
	worker_assign := make(map[string]uint)
	for _, i := range makeRange(0, len(cut_index)-1) {
		if i == 0 {
			for _, j := range makeRange(int(cut_index[i]), int(cut_index[i+1]+1)) {
				worker_assign[line[j].name] = uint(worker_index)
			}
		} else {
			for _, j := range makeRange(int(cut_index[i]+1), int(cut_index[i+1]+1)) {
				worker_assign[line[j].name] = uint(worker_index)
			}
		}

		worker_index += 1

		present_cluster_size := uint(0)
		if i == 0 {
			present_cluster_size += C[0][vertex_num-1][cut_index[1]]
		} else if i == uint(len(cut_index)-2) {
			present_cluster_size += C[0][vertex_num-1][cut_index[i]]
		} else {
			present_cluster_size += C[0][cut_index[i+1]][cut_index[i]]
			present_cluster_size += C[cut_index[i]][vertex_num-1][cut_index[i+1]]
		}
		present_cluster_size += D[cut_index[i]][cut_index[i+1]]
		present_cluster_size += B[cut_index[i]][cut_index[i+1]]
		cluster_size = append(cluster_size, present_cluster_size)
		if present_cluster_size > uint(max_cluster_size) {
			max_cluster_size = int(present_cluster_size)
		}
	}

	mean := float64(0)
	variance := float64(0)
	for _, i := range makeRange(0, len(cluster_size)) {
		mean += float64(cluster_size[i]) / float64(len(cluster_size))
		variance += math.Pow(float64(cluster_size[i])-float64(mean), 2)
	}
	variance /= float64(len(cluster_size))
	standard_deviation := math.Sqrt(variance)
	coefficient_of_variation := standard_deviation / mean
	log.Println("The cut sizes is:", cluster_size)
	log.Println("The mean cut size is:", mean)
	log.Println("The max cut size is:", max_cluster_size)
	log.Println("The coefficient of variance is:", coefficient_of_variation)
	return worker_assign
}

func Combination(af Affinity, partition_number uint, interval_len uint, rank_swap bool) []element {
	q := make([]uint, 0)
	line := af.linear_embed()
	for _, i := range makeRange(0, int(partition_number)+1) {
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
	for _, _ = range makeRange(0, int(vertex)) {
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

func Random_mock(scale uint, partition_number uint, rank_swap bool, threshold uint) {
	edges := make_random_graph(scale)
	hash_edges := get_hash_edges(edges)
	new_edges := find_commen_neighbors(edges)
	af := make_cluster(0.4, new_edges, threshold, true, true)
	line_after_swap := Combination(af, partition_number, uint(math.Sqrt(float64(scale/partition_number))), rank_swap)

	DynamicProgram(line_after_swap, partition_number, hash_edges)
}

func generate_edges(message rpctest.Message) ([]Edge, uint) {
	topo := message.Command.EmunetCreation.Emunet
	edges := make([]Edge, 0)
	for _, edge := range topo.Links {
		edges = append(edges, Edge{
			start: element(Node{
				name:   edge.Node1.Name,
				weight: 1,
			}),
			end: element(Node{
				name:   edge.Node2.Name,
				weight: 1,
			}),
		})
	}
	return edges, uint(len(topo.Pods))
}

func AffinityClusterPartition(message rpctest.Message, partition_number uint) map[string]uint {
	edges, scale := generate_edges(message)
	hash_edges := get_hash_edges(edges)
	new_edges := find_commen_neighbors(edges)
	threshold := scale / partition_number
	af := make_cluster(0.4, new_edges, threshold, true, true)
	line_after_swap := Combination(af, partition_number, uint(math.Sqrt(float64(scale/partition_number))), true)

	return DynamicProgram(line_after_swap, partition_number, hash_edges)
}
