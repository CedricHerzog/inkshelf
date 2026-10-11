package servers

import (
	"slices"
	"testing"
)

func TestFold(t *testing.T) {
	if got := Fold("Œufs brouillés à l'Été"); got != "Oeufs brouilles a l'Ete" {
		t.Fatalf("Fold: %q", got)
	}
}

func TestSortKey(t *testing.T) {
	tests := []struct {
		title, want string
	}{
		{"Le Trône de Fer", "Trone de Fer"},
		{"la Horde du Contrevent", "Horde du Contrevent"},
		{"LES MISÉRABLES", "MISERABLES"},
		{"L'Assassin royal", "Assassin royal"},
		{"L’Écume des jours", "Ecume des jours"},
		{"L'homme qui rit", "homme qui rit"},
		{"L'Kanji", "L'Kanji"}, // no elision before a consonant
		{"Un sac de billes", "sac de billes"},
		{"Une vie", "vie"},
		{"Des fleurs pour Algernon", "fleurs pour Algernon"},
		{"The Hobbit", "Hobbit"},
		{"A Game of Thrones", "Game of Thrones"},
		{"An Ember in the Ashes", "Ember in the Ashes"},
		{"À la croisée des mondes", "A la croisee des mondes"}, // a preposition
		{"A la croisée des mondes", "A la croisee des mondes"}, // an "à" without its accent
		{"Theory of Everything", "Theory of Everything"},       // no space after "The"
		{"Lesson", "Lesson"},
		{"The", "The"},
		{"Les ", "Les"}, // would be empty
		{"Dune – Tome 2", "Dune - Tome 2"},
		{"  Le  Seigneur des anneaux ", "Seigneur des anneaux"},
	}
	for _, test := range tests {
		if got := SortKey(test.title); got != test.want {
			t.Errorf("SortKey(%q) = %q, want %q", test.title, got, test.want)
		}
	}
}

func TestCompareTitles(t *testing.T) {
	titles := []string{
		"Dune - Tome 10",
		"The Zeta",
		"Dune - Tome 3",
		"Le Trône de Fer",
		"Dune – Tome 2",
		"Akira",
		"Dune - Tome 1",
		"Les Étoiles",
		"Zorro",
		"eragon",
	}
	slices.SortFunc(titles, CompareTitles)
	want := []string{
		"Akira",
		"Dune - Tome 1",
		"Dune – Tome 2",
		"Dune - Tome 3",
		"Dune - Tome 10",
		"eragon",
		"Les Étoiles",
		"Le Trône de Fer",
		"The Zeta",
		"Zorro",
	}
	if !slices.Equal(titles, want) {
		t.Errorf("sorted %q, want %q", titles, want)
	}
	if CompareTitles("Le Hobbit", "Hobbit") == 0 {
		t.Error("two titles of the same key compare equal")
	}
}
