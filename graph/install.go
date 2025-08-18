package graph

type Install struct {
	g *Graph
}

func (i *Install) SetPlayer(user Data) {
	i.g.nodes[user.GetUid()] = NewPlayer(user)
}
func (i *Install) SetFriend(u1, u2 string) error {
	p1, err := i.g.load(u1)
	if err != nil {
		return err
	}
	p2, err := i.g.load(u2)
	if err != nil {
		return err
	}
	p1.Add(p2)
	p2.Add(p1)
	return nil
}
