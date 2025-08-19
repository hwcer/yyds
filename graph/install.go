package graph

type Install struct {
	g *Graph
}

func (i *Install) SetPlayer(p Player) {
	i.g.nodes[p.GetUid()] = newNode(p)
}
func (i *Install) SetFriend(uid string, f Friend) error {
	p, err := i.g.load(uid)
	if err != nil {
		return err
	}
	if _, err = i.g.load(f.GetUid()); err != nil {
		return err
	}
	p.Add(f)
	return nil
}
