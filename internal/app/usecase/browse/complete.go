package browse

import (
	"slices"

	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// Vocabulary is what a value of a task can be in the stores a list shows, for
// the completion of the shell. Each list is sorted and has no repeats.
type Vocabulary struct {
	Statuses []string
	Props    []string // the custom properties
	// Priorities and Labels are the values of the lists of the schemas.
	Priorities []string
	Labels     []string
	Members    []string // account IDs
}

// ReadVocabulary reads the vocabulary of the stores of a list.
func ReadVocabulary(ss *usecase.Session) (Vocabulary, error) {
	set, err := listSet(ss)
	if err != nil {
		return Vocabulary{}, err
	}
	var v Vocabulary
	for _, m := range set.Stores {
		sc := m.Store.Schema()
		for _, st := range sc.Statuses {
			v.Statuses = append(v.Statuses, st.Name)
		}
		for _, p := range sc.Props {
			v.Props = append(v.Props, p.Name)
		}
		v.Priorities = append(v.Priorities, sc.Priorities...)
		v.Labels = append(v.Labels, sc.Labels...)
		ms, err := replicate.ReadMembers(m.Store)
		if err != nil {
			return Vocabulary{}, usecase.Classify(err)
		}
		v.Members = append(v.Members, ms.Authors...)
		for _, c := range ms.Collaborators {
			v.Members = append(v.Members, c.ID)
		}
	}
	for _, l := range []*[]string{&v.Statuses, &v.Props, &v.Priorities, &v.Labels, &v.Members} {
		slices.Sort(*l)
		*l = slices.Compact(*l)
	}
	return v, nil
}
