package replicate

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// mirrorsHeldFile is .hedgehug/local/mirrors-held.json: the URLs of the
// mirrors that had the history a rebuild removed. A sync leaves them alone
// until they were rewritten as origin was, so that the removed content does
// not come back through them.
const mirrorsHeldFile = "mirrors-held.json"

// fetchMarker is .hedgehug/local/origin-before-fetch: the value of the
// tracking ref of origin before a fetch, kept until checkOrigin has compared
// the new head with it. A sync that died in between left the ref on the
// fetched head, and the force push in it would look like a history the store
// follows.
const fetchMarker = "origin-before-fetch"

// readHeld reads mirrors-held.json; none when there is no file.
func readHeld(s *store.Store) ([]string, error) {
	b, err := s.ReadLocal(mirrorsHeldFile)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var urls []string
	if err := json.Unmarshal(b, &urls); err != nil {
		return nil, fmt.Errorf("%s: %w", mirrorsHeldFile, err)
	}
	return urls, nil
}

// writeHeld writes mirrors-held.json; with no URL it removes the file.
func writeHeld(s *store.Store, urls []string) error {
	if len(urls) == 0 {
		return s.RemoveLocal(mirrorsHeldFile)
	}
	b, err := json.Marshal(urls)
	if err != nil {
		return err
	}
	return s.WriteLocal(mirrorsHeldFile, append(b, '\n'))
}

// holdMirrors adds urls to the mirrors held; with none it writes nothing.
func holdMirrors(s *store.Store, urls []string) error {
	if len(urls) == 0 {
		return nil
	}
	held, err := readHeld(s)
	if err != nil {
		return err
	}
	for _, u := range urls {
		if !slices.Contains(held, u) {
			held = append(held, u)
		}
	}
	return writeHeld(s, held)
}

// syncHeld is the mirrors held at the start of a sync, urls being the
// mirrors the config has now; one the config no longer has is dropped.
func syncHeld(s *store.Store, urls []string) ([]string, error) {
	held, err := readHeld(s)
	if err != nil || len(held) == 0 {
		return nil, err
	}
	kept := slices.DeleteFunc(slices.Clone(held), func(u string) bool { return !slices.Contains(urls, u) })
	if len(kept) != len(held) {
		if err := writeHeld(s, kept); err != nil {
			return nil, err
		}
	}
	return kept, nil
}

// mirrorHeld tells whether the mirror at url, which is held, is still to be
// left alone. It is not when its main is a commit the store has (or it has
// no main): the mirror was rewritten as origin was, and is let go. An error
// is that of asking the mirror. While a rebuild is not over it stays held
// without asking: the old head may still be on the disk (a prune that keeps
// failing), and a mirror that has it would look rewritten.
func mirrorHeld(s *store.Store, name, url string, held *[]string) (bool, error) {
	if store.RebuildPhaseIn(s.Dir()) != store.RebuildNone {
		return true, nil
	}
	r := s.Repo()
	main, err := r.RemoteMain(name)
	if err != nil {
		return true, err
	}
	known := main.IsZero()
	if !known {
		if known, err = r.HasCommit(main); err != nil {
			return true, err
		}
	}
	if !known {
		return true, nil
	}
	*held = slices.DeleteFunc(*held, func(u string) bool { return u == url })
	return false, writeHeld(s, *held)
}

// heldMessage is the warning for a mirror that is held.
func heldMessage(url string) error {
	return fmt.Errorf("the mirror %s still has the old history; rewrite it as origin was", url)
}

// beginFetch returns what the store tracks of origin, the value of a fetch
// that died before checkOrigin if there is one, and records it.
func beginFetch(s *store.Store) (plumbing.Hash, error) {
	r := s.Repo()
	if b, err := s.ReadLocal(fetchMarker); err == nil {
		if h := plumbing.NewHash(strings.TrimSpace(string(b))); !h.IsZero() && h.String() == strings.TrimSpace(string(b)) {
			return h, r.SetTracking("origin", h)
		}
		// A marker that does not parse says nothing: go on from the ref.
		if err := s.RemoveLocal(fetchMarker); err != nil {
			return plumbing.ZeroHash, err
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return plumbing.ZeroHash, err
	}
	tracked, err := r.RemoteHead("origin")
	if err != nil || tracked.IsZero() {
		return tracked, err
	}
	return tracked, s.WriteLocal(fetchMarker, []byte(tracked.String()+"\n"))
}

// endFetch removes the marker beginFetch wrote.
func endFetch(s *store.Store) error { return s.RemoveLocal(fetchMarker) }

// withEnd is err, with the failure of endFetch joined to it; err itself when
// endFetch worked.
func withEnd(err, endErr error) error {
	switch {
	case endErr == nil:
		return err
	case err == nil:
		return endErr
	}
	return errors.Join(err, endErr)
}
