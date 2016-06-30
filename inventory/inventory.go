package inventory

import (
	"database/sql"

	// import for side effects (load the sqlite3 driver)
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/context"

	"github.com/bh107/tapr/util/mtx"
	"github.com/bh107/tapr/util/proc"
)

type Inventory struct {
	*proc.Proc

	db *sql.DB
}

func New(dbname string) (*Inventory, error) {
	// open inventory database
	handle, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return nil, err
	}

	inv := &Inventory{db: handle}

	inv.Proc = proc.Create(inv)

	return inv, nil
}

func (inv *Inventory) ProcessName() string {
	return "inventory"
}

func (inv *Inventory) Handle(ctx context.Context, req proc.HandleFn) error {
	return req(ctx)
}

func (inv *Inventory) Locate(ctx context.Context, vol *mtx.Volume) (string, error) {
	var libname string

	req := func(ctx context.Context) error {
		row := inv.db.QueryRow(`SELECT library FROM volume WHERE serial = ?`, vol.Serial)

		if err := row.Scan(&libname); err != nil {
			return err
		}

		return nil
	}

	if err := inv.Wait(ctx, req); err != nil {
		return "", err
	}

	return libname, nil
}

func (inv *Inventory) Volumes(ctx context.Context, libname string) ([]*mtx.Volume, error) {
	var vols []*mtx.Volume

	req := func(ctx context.Context) error {
		rows, err := inv.db.Query(`
			SELECT serial
			FROM volume
			WHERE library = ?`,
			libname,
		)

		if err != nil {
			return err
		}

		for rows.Next() {
			var serial string
			if err := rows.Scan(&serial); err != nil {
				return err
			}

			vols = append(vols, &mtx.Volume{Serial: serial})
		}

		if err := rows.Err(); err != nil {
			return err
		}

		return nil
	}

	if err := inv.Wait(ctx, req); err != nil {
		return nil, err
	}

	return vols, nil
}

func (inv *Inventory) GetScratch(ctx context.Context, libname string) (*mtx.Volume, error) {
	var serial string
	var slot int

	req := func(ctx context.Context) error {
		tx, err := inv.db.Begin()
		if err != nil {
			return err
		}

		row := inv.db.QueryRow(`
			SELECT serial, slot
			FROM volume
			WHERE status = "scratch"
			  AND library = ?
		  	AND slot is NOT NULL
			LIMIT 1`,
			libname,
		)

		if err := row.Scan(&serial, &slot); err != nil {
			if err := tx.Rollback(); err != nil {
				return err
			}

			return err
		}

		_, err = tx.Exec(`
		UPDATE volume
		SET status = "alloc"
		WHERE serial = ?
			AND status = "scratch"
	`, serial)

		if err != nil {
			if err := tx.Rollback(); err != nil {
				return err
			}

			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}

		return nil
	}

	if err := inv.Wait(ctx, req); err != nil {
		return nil, err
	}

	return &mtx.Volume{Serial: serial, Home: slot}, nil
}

func (inv *Inventory) Close(ctx context.Context) error {
	req := func(ctx context.Context) error {
		return inv.db.Close()
	}

	return inv.Wait(ctx, req)
}

func (inv *Inventory) Audit(ctx context.Context, status *mtx.StatusInfo, libname string) error {
	req := func(ctx context.Context) error {
		// update volume locations
		for _, slot := range status.Slots {
			if slot.Vol != nil {
				// try to insert the row
				_, err := inv.db.Exec(`
					INSERT OR IGNORE INTO volume (serial, slot, status, library)
					VALUES (?, ?, ?, ?)`,
					slot.Vol.Serial, slot.Num, "scratch", libname,
				)

				if err != nil {
					return err
				}

				// if it was ignored, make sure slot is updated... but EXPLICITLY
				// DO NOT RISK UPDATING STATUS TO SCRATCH FUCKHEAD!
				_, err = inv.db.Exec(`
					UPDATE volume
					SET slot = ?, library = ?
					WHERE serial = ?`,
					slot.Num, libname, slot.Vol.Serial,
				)

				if err != nil {
					return err
				}
			}
		}

		return nil
	}

	return inv.Wait(ctx, req)
}
