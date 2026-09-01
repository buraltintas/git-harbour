package game

import (
	"errors"
	"math"
	"sort"
)

// Contribution Fleet v3 balancing constants live together so the product can
// tune combat without scattering rules through handlers or UI projections.
const (
	ContributionFleetRuleset      = "contribution_fleet_v3"
	ContributionBattleshipRuleset = "contribution_battleship_v4"
	MinimumFleetCapacity          = 3
	MaximumFleetCapacity          = 14
	ReservePower                  = 1.0
	CombatScale                   = 10.0
	MinimumWinChance              = 0.15
	MaximumWinChance              = 0.85
	combatRollPrecision           = 1_000_000
)

// ResolveDeploymentShot implements the familiar Battleship action: the player
// chooses only an opponent coordinate. A deployed unit at that coordinate is
// hit and eliminated; an empty coordinate is a miss. Unit power is not an
// attack input in this ruleset.
func ResolveDeploymentShot(units []FleetUnit, previous []TargetShot, target Coord) (TargetShot, []FleetUnit, error) {
	if !InBounds(target) {
		return TargetShot{}, units, errors.New("shot out of bounds")
	}
	for _, shot := range previous {
		if shot.Coord == target {
			return TargetShot{}, units, errors.New("duplicate shot")
		}
	}
	next := append([]FleetUnit(nil), units...)
	shot := TargetShot{Coord: target, Result: "miss"}
	for i := range next {
		if next[i].Coord == target && next[i].Alive {
			next[i].Alive = false
			next[i].Exposed = true
			shot.Result = "hit"
			break
		}
	}
	return shot, next, nil
}

type FleetWindow struct {
	StartIndex         int
	Cells              []Cell
	TotalContributions int
	ActiveDays         int
	ContributionPower  float64
	FleetCapacity      int
	PeakCount          int
	PeakDate           string
	MaxDeployedPower   float64
}

type FleetUnit struct {
	Coord
	Kind              string  `json:"kind"`
	ContributionCount int     `json:"contributionCount,omitempty"`
	Power             float64 `json:"power"`
	Level             int     `json:"level"`
	Alive             bool    `json:"alive"`
	Exposed           bool    `json:"exposed"`
}

type DeploymentChoice struct {
	Coord
	Kind string `json:"kind"`
}

type FleetAction struct {
	Actor         string  `json:"actor"`
	Attacker      Coord   `json:"attacker"`
	Target        Coord   `json:"target"`
	Result        string  `json:"result"`
	AttackerWon   bool    `json:"attackerWon,omitempty"`
	Probability   float64 `json:"probability,omitempty"`
	Roll          float64 `json:"roll,omitempty"`
	AttackerPower float64 `json:"attackerPower,omitempty"`
	DefenderPower float64 `json:"defenderPower,omitempty"`
}

func DayPower(contributions int) float64 {
	if contributions <= 0 {
		return 0
	}
	return 1 + math.Log2(1+float64(contributions))
}

func CombatLevel(power float64) int {
	switch {
	case power <= ReservePower:
		return 0
	case power < 4:
		return 1
	case power < 7:
		return 2
	case power < 10:
		return 3
	default:
		return 4
	}
}

// FleetCapacity uses a rounded sqrt curve calibrated to approximately
// 0→3, 1→4, 10→7, 30→10, 50→12 and 70→14 units.
func FleetCapacity(activeDays int) int {
	if activeDays < 0 {
		activeDays = 0
	}
	if activeDays > BoardCells {
		activeDays = BoardCells
	}
	capacity := int(math.Floor(3 + math.Sqrt(12*float64(activeDays)/7) + 0.5))
	if capacity < MinimumFleetCapacity {
		return MinimumFleetCapacity
	}
	if capacity > MaximumFleetCapacity {
		return MaximumFleetCapacity
	}
	return capacity
}

func SummarizeFleetWindow(cells []Cell, startIndex int) (FleetWindow, error) {
	if !contiguous(cells) {
		return FleetWindow{}, errors.New("window must be a contiguous week-aligned ten-week period")
	}
	w := FleetWindow{StartIndex: startIndex, Cells: append([]Cell(nil), cells...)}
	powers := make([]float64, 0, BoardCells)
	for _, cell := range cells {
		if cell.ContributionCount < 0 {
			return FleetWindow{}, errors.New("contribution count cannot be negative")
		}
		w.TotalContributions += cell.ContributionCount
		power := DayPower(cell.ContributionCount)
		w.ContributionPower += power
		if cell.ContributionCount > 0 {
			w.ActiveDays++
			powers = append(powers, power)
			if cell.ContributionCount > w.PeakCount {
				w.PeakCount, w.PeakDate = cell.ContributionCount, cell.Date
			}
		}
	}
	w.FleetCapacity = FleetCapacity(w.ActiveDays)
	sort.Sort(sort.Reverse(sort.Float64Slice(powers)))
	for i := 0; i < w.FleetCapacity; i++ {
		if i < len(powers) {
			w.MaxDeployedPower += powers[i]
		} else {
			w.MaxDeployedPower += ReservePower
		}
	}
	return w, nil
}

func FleetWindowAt(days []Cell, start string) (FleetWindow, error) {
	// Imported GitHub history can begin on any weekday, so do not assume index
	// zero is the beginning of a week. SummarizeFleetWindow validates alignment.
	for i := 0; i+BoardCells <= len(days); i++ {
		cells := days[i : i+BoardCells]
		if len(cells) > 0 && cells[0].Date == start {
			return SummarizeFleetWindow(cells, i)
		}
	}
	return FleetWindow{}, errors.New("selected contribution window is invalid")
}

// SelectFleetOpponentWindow chooses a real, distinct window. Non-overlapping
// alternatives are preferred when present; selection within the pool is random.
func SelectFleetOpponentWindow(days []Cell, player FleetWindow, r Rander) (FleetWindow, error) {
	if r == nil {
		return FleetWindow{}, errors.New("random source is required")
	}
	all, nonOverlapping := []FleetWindow{}, []FleetWindow{}
	for i := 0; i+BoardCells <= len(days); i++ {
		if i == player.StartIndex {
			continue
		}
		candidate, err := SummarizeFleetWindow(days[i:i+BoardCells], i)
		if err != nil {
			continue
		}
		all = append(all, candidate)
		if absTarget(i-player.StartIndex) >= BoardCells {
			nonOverlapping = append(nonOverlapping, candidate)
		}
	}
	if len(all) == 0 {
		return FleetWindow{}, errors.New("contribution history has no distinct opponent window")
	}
	pool := all
	if len(nonOverlapping) > 0 {
		pool = nonOverlapping
	}
	i, err := r.Intn(len(pool))
	if err != nil || i < 0 || i >= len(pool) {
		return FleetWindow{}, errors.New("could not choose opponent contribution window")
	}
	return pool[i], nil
}

func ValidateDeployment(board []Cell, choices []DeploymentChoice) ([]FleetUnit, error) {
	window, err := SummarizeFleetWindow(board, 0)
	if err != nil {
		return nil, err
	}
	if len(choices) != window.FleetCapacity {
		return nil, errors.New("deployment must fill fleet capacity")
	}
	seen := map[Coord]bool{}
	selectedContributions := 0
	units := make([]FleetUnit, 0, len(choices))
	for _, choice := range choices {
		if !InBounds(choice.Coord) || seen[choice.Coord] {
			return nil, errors.New("deployment contains an invalid or duplicate coordinate")
		}
		seen[choice.Coord] = true
		cell := board[choice.X*Height+choice.Y]
		switch choice.Kind {
		case "contribution":
			if cell.ContributionCount <= 0 {
				return nil, errors.New("contribution unit must remain on its real active date")
			}
			power := DayPower(cell.ContributionCount)
			units = append(units, FleetUnit{Coord: choice.Coord, Kind: choice.Kind, ContributionCount: cell.ContributionCount, Power: power, Level: CombatLevel(power), Alive: true})
			selectedContributions++
		case "reserve":
			if cell.ContributionCount != 0 {
				return nil, errors.New("reserve unit must use an inactive cell")
			}
			units = append(units, FleetUnit{Coord: choice.Coord, Kind: choice.Kind, Power: ReservePower, Level: CombatLevel(ReservePower), Alive: true})
		default:
			return nil, errors.New("unknown fleet unit kind")
		}
	}
	requiredContributions := window.ActiveDays
	if requiredContributions > window.FleetCapacity {
		requiredContributions = window.FleetCapacity
	}
	if selectedContributions != requiredContributions {
		return nil, errors.New("deployment must use the required number of contribution-backed units")
	}
	return units, nil
}

func RandomDeployment(board []Cell, r Rander) ([]FleetUnit, error) {
	if r == nil {
		return nil, errors.New("random source is required")
	}
	w, err := SummarizeFleetWindow(board, 0)
	if err != nil {
		return nil, err
	}
	active, empty := []Coord{}, []Coord{}
	for x := 0; x < Width; x++ {
		for y := 0; y < Height; y++ {
			coord := Coord{X: x, Y: y}
			if board[x*Height+y].ContributionCount > 0 {
				active = append(active, coord)
			} else {
				empty = append(empty, coord)
			}
		}
	}
	choices := make([]DeploymentChoice, 0, w.FleetCapacity)
	for len(choices) < w.FleetCapacity && len(active) > 0 {
		i, e := r.Intn(len(active))
		if e != nil {
			return nil, e
		}
		choices = append(choices, DeploymentChoice{Coord: active[i], Kind: "contribution"})
		active = append(active[:i], active[i+1:]...)
	}
	for len(choices) < w.FleetCapacity {
		i, e := r.Intn(len(empty))
		if e != nil {
			return nil, e
		}
		choices = append(choices, DeploymentChoice{Coord: empty[i], Kind: "reserve"})
		empty = append(empty[:i], empty[i+1:]...)
	}
	return ValidateDeployment(board, choices)
}

func WinProbability(attackerPower, defenderPower float64) float64 {
	p := 1 / (1 + math.Pow(10, (defenderPower-attackerPower)/CombatScale))
	return math.Max(MinimumWinChance, math.Min(MaximumWinChance, p))
}

func AliveCount(units []FleetUnit) int {
	n := 0
	for _, unit := range units {
		if unit.Alive {
			n++
		}
	}
	return n
}

func ResolveFleetAction(attacker, defender []FleetUnit, previous []FleetAction, actor string, attackerCoord, target Coord, r Rander) (FleetAction, []FleetUnit, []FleetUnit, error) {
	if r == nil || !InBounds(attackerCoord) || !InBounds(target) {
		return FleetAction{}, attacker, defender, errors.New("invalid combat action")
	}
	retargetable := false
	for _, unit := range defender {
		if unit.Coord == target && unit.Alive && unit.Exposed {
			retargetable = true
			break
		}
	}
	for _, action := range previous {
		if action.Actor == actor && action.Target == target && !retargetable {
			return FleetAction{}, attacker, defender, errors.New("duplicate target")
		}
	}
	a := append([]FleetUnit(nil), attacker...)
	d := append([]FleetUnit(nil), defender...)
	attackerIndex := -1
	for i := range a {
		if a[i].Coord == attackerCoord && a[i].Alive {
			attackerIndex = i
			break
		}
	}
	if attackerIndex < 0 {
		return FleetAction{}, attacker, defender, errors.New("attacker must be a surviving deployed unit")
	}
	a[attackerIndex].Exposed = true
	event := FleetAction{Actor: actor, Attacker: attackerCoord, Target: target, Result: "miss", AttackerPower: a[attackerIndex].Power}
	defenderIndex := -1
	for i := range d {
		if d[i].Coord == target && d[i].Alive {
			defenderIndex = i
			break
		}
	}
	if defenderIndex < 0 {
		return event, a, d, nil
	}
	d[defenderIndex].Exposed = true
	event.Result = "clash"
	event.DefenderPower = d[defenderIndex].Power
	event.Probability = WinProbability(event.AttackerPower, event.DefenderPower)
	roll, err := r.Intn(combatRollPrecision)
	if err != nil {
		return FleetAction{}, attacker, defender, err
	}
	event.Roll = float64(roll) / combatRollPrecision
	event.AttackerWon = event.Roll < event.Probability
	if event.AttackerWon {
		d[defenderIndex].Alive = false
	} else {
		a[attackerIndex].Alive = false
	}
	return event, a, d, nil
}

// NextComputerAction receives only its own fleet and auditable public combat
// history. It never receives the hidden player deployment.
func NextComputerAction(computer []FleetUnit, history []FleetAction, r Rander) (Coord, Coord, error) {
	alive := []Coord{}
	for _, unit := range computer {
		if unit.Alive {
			alive = append(alive, unit.Coord)
		}
	}
	if len(alive) == 0 {
		return Coord{}, Coord{}, errors.New("computer has no surviving attacker")
	}
	ai, err := r.Intn(len(alive))
	if err != nil {
		return Coord{}, Coord{}, err
	}
	used := map[Coord]bool{}
	knownAlive := map[Coord]bool{}
	for _, event := range history {
		if event.Actor == "ai" {
			used[event.Target] = true
		}
		if event.Actor == "player" {
			knownAlive[event.Attacker] = !(event.Result == "clash" && !event.AttackerWon)
		} else if event.Result == "clash" {
			knownAlive[event.Target] = !event.AttackerWon
		}
	}
	preferred := []Coord{}
	for coord, isAlive := range knownAlive {
		if isAlive {
			preferred = append(preferred, coord)
		}
	}
	pool := preferred
	if len(pool) == 0 {
		for x := 0; x < Width; x++ {
			for y := 0; y < Height; y++ {
				c := Coord{X: x, Y: y}
				if !used[c] {
					pool = append(pool, c)
				}
			}
		}
	}
	if len(pool) == 0 {
		return Coord{}, Coord{}, errors.New("computer has no valid target")
	}
	ti, err := r.Intn(len(pool))
	if err != nil {
		return Coord{}, Coord{}, err
	}
	return alive[ai], pool[ti], nil
}
