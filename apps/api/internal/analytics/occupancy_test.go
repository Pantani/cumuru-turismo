package analytics_test

import (
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

func TestComputeOccupancyKeepsOwnExactAndVillageBanded(t *testing.T) {
	t.Parallel()

	// 62 pessoas-dia em 10 leitos por 10 dias = 62%; a vila, 710 em 100 por 10.
	occupancy := analytics.ComputeOccupancy(analytics.OccupancyInput{
		Days:              10,
		OwnCapacity:       10,
		OwnPersonDays:     62,
		VillageCapacity:   100,
		VillagePersonDays: 710,
	}, true)
	if occupancy.Own == nil || *occupancy.Own != 62 {
		t.Fatalf("own = %v, want 62", occupancy.Own)
	}
	// 71% cai na banda de cinco pontos: 70.
	if occupancy.Village == nil || *occupancy.Village != 70 {
		t.Fatalf("village = %v, want 70", occupancy.Village)
	}
}

func TestComputeOccupancyWithdrawsTheVillageWhenTheComparisonIsClosed(t *testing.T) {
	t.Parallel()

	occupancy := analytics.ComputeOccupancy(analytics.OccupancyInput{
		Days:              10,
		OwnCapacity:       10,
		OwnPersonDays:     62,
		VillageCapacity:   100,
		VillagePersonDays: 710,
	}, false)
	if occupancy.Own == nil {
		t.Fatal("own occupancy survives a closed comparison")
	}
	if occupancy.Village != nil {
		t.Fatalf("village = %v, want absent", occupancy.Village)
	}
}

func TestComputeOccupancyWithoutDeclaredCapacity(t *testing.T) {
	t.Parallel()

	occupancy := analytics.ComputeOccupancy(analytics.OccupancyInput{
		Days: 10, OwnCapacity: 0, OwnPersonDays: 40,
		VillageCapacity: 100, VillagePersonDays: 500,
	}, true)
	if occupancy.Own != nil {
		t.Fatal("no declared capacity is no denominator, not a zero rate")
	}
	if occupancy.Village == nil || *occupancy.Village != 50 {
		t.Fatalf("village = %v, want 50", occupancy.Village)
	}
}

func TestComputeOccupancyReportsOversubscriptionInsteadOfHidingIt(t *testing.T) {
	t.Parallel()

	// Capacidade declarada menor que a operação real é problema de cadastro, e o
	// painel de qualidade existe para isso: aparar em 100 esconderia o defeito.
	occupancy := analytics.ComputeOccupancy(analytics.OccupancyInput{
		Days: 10, OwnCapacity: 4, OwnPersonDays: 50,
		VillageCapacity: 100, VillagePersonDays: 500,
	}, true)
	if occupancy.Own == nil || *occupancy.Own != 125 {
		t.Fatalf("own = %v, want 125", occupancy.Own)
	}
}

func TestComputeOccupancyWithoutDaysInTheWindow(t *testing.T) {
	t.Parallel()

	occupancy := analytics.ComputeOccupancy(analytics.OccupancyInput{
		Days: 0, OwnCapacity: 10, OwnPersonDays: 0,
		VillageCapacity: 100, VillagePersonDays: 0,
	}, true)
	if occupancy.Own != nil || occupancy.Village != nil {
		t.Fatal("an empty window has no rate on either side")
	}
}
