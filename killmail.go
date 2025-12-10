package killfeed

import (
	"time"

	"github.com/antihax/goesi/esi"
)

type Killmail = esi.GetKillmailsKillmailIdKillmailHashOk

type KillmailZkb struct {
	Hash           string   `json:"hash,omitzero"`
	LocationID     int      `json:"locationID,omitzero"`
	FittedValue    float64  `json:"fittedValue,omitzero"`
	DroppedValue   float64  `json:"droppedValue,omitzero"`
	DestroyedValue float64  `json:"destroyedValue,omitzero"`
	TotalValue     float64  `json:"totalValue,omitzero"`
	Points         int      `json:"points,omitzero"`
	Npc            bool     `json:"npc,omitzero"`
	Solo           bool     `json:"solo,omitzero"`
	Awox           bool     `json:"awox,omitzero"`
	Labels         []string `json:"labels,omitzero"`
	Href           string   `json:"href,omitzero"`
}

type CombinedKillmail struct {
	// These fields need to be copied here, because extending would not work otherwise because
	// goesi is using easyjson custom marshalers
	Attackers     []esi.GetKillmailsKillmailIdKillmailHashAttacker `json:"attackers,omitempty"`       /* attackers array */
	KillmailId    int32                                            `json:"killmail_id,omitempty"`     /* ID of the killmail */
	KillmailTime  time.Time                                        `json:"killmail_time,omitzero"`    /* Time that the victim was killed and the killmail generated  */
	MoonId        int32                                            `json:"moon_id,omitempty"`         /* Moon if the kill took place at one */
	SolarSystemId int32                                            `json:"solar_system_id,omitempty"` /* Solar system that the kill took place in  */
	Victim        esi.GetKillmailsKillmailIdKillmailHashVictim     `json:"victim,omitzero"`
	WarId         int32                                            `json:"war_id,omitempty"` /* War if the killmail is generated in relation to an official war  */

	Zkb KillmailZkb `json:"zkb"`
}
