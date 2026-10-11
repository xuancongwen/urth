package world

import (
	"urth/internal/item"
	"urth/internal/output"
)

// Rewards: what a player gains in one moment (a kill and its looting, a
// quest handed in, an expedition's return) is gathered as it happens and
// sent at the end of the tick as one Reward message, so a client can show
// the loop's payoff in one place instead of leaving it in scrolling text.

// rewardFor returns p's reward for this tick, starting one if needed.
func rewardFor(c *Character) *output.RewardData {
	if c == nil || c.player == nil {
		return nil
	}
	p := c.player
	if p.reward == nil {
		p.reward = &output.RewardData{Items: []output.GearItem{}}
	}
	return p.reward
}

// rewardTitle names what earned the reward, if nothing has yet.
func rewardTitle(c *Character, title string) {
	if r := rewardFor(c); r != nil && r.Title == "" {
		r.Title = title
	}
}

// rewardItem adds an item the player just took to the reward, as the bag
// shows it (with the reference a command takes, and its numbers), so a
// client can offer to wear it.
// rewardKill records a foe whose death paid the reward.
func rewardKill(c *Character, name string) {
	if r := rewardFor(c); r != nil {
		r.Kills = append(r.Kills, name)
	}
}

func (w *World) rewardItem(c *Character, it *item.Item) {
	r := rewardFor(c)
	if r == nil {
		return
	}
	g := gearItem(it, itemRef(c.Inventory, w.searchPool(c.player), it))
	g.Slot = string(it.Proto.WearSlot())
	r.Items = append(r.Items, g)
}

// queueReward sends p's gathered reward, if any.
func (w *World) queueReward(p *Player) {
	if p.reward == nil {
		return
	}
	r := p.reward
	p.reward = nil
	if r.XP == 0 && r.Silver == 0 && r.QuestPoints == 0 && len(r.Items) == 0 && r.Level == 0 {
		return
	}
	p.SendMsg(output.Message{Type: output.Reward, Data: *r})
}
