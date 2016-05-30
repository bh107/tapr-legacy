package inventory

import (
	"database/sql"

	// import for side effects (load the sqlite3 driver)
	"github.com/bh107/tapr/mtx"
	_ "github.com/mattn/go-sqlite3"
)

type Inventory struct {
	db *sql.DB
}

func New(dbname string) (*Inventory, error) {
	// open inventory database
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return nil, err
	}

	return &Inventory{db: db}, nil
}

func (inv *Inventory) Locate(vol *mtx.Volume) (string, error) {
	row := inv.db.QueryRow(`SELECT library FROM volume WHERE serial = ?`, vol.Serial)

	var libname string

	if err := row.Scan(&libname); err != nil {
		return "", err
	}

	return libname, nil
}

func (inv *Inventory) Volumes(libname string) ([]*mtx.Volume, error) {
	rows, err := inv.db.Query(`
		SELECT serial
		FROM volume
		WHERE library = ?
	`, libname)
	if err != nil {
		return nil, err
	}

	var vols []*mtx.Volume
	for rows.Next() {
		var serial string
		if err := rows.Scan(&serial); err != nil {
			return nil, err
		}

		vols = append(vols, &mtx.Volume{Serial: serial})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return vols, nil
}

func (inv *Inventory) GetScratch(libname string) (*mtx.Volume, error) {
	tx, err := inv.db.Begin()
	if err != nil {
		return nil, err
	}

	row := inv.db.QueryRow(`
		SELECT serial, slot
		FROM volume
		WHERE status = "scratch"
		  AND library = ?
		  AND slot is NOT NULL
		LIMIT 1
	`, libname)

	var serial string
	var slot int

	if err := row.Scan(&serial, &slot); err != nil {
		if err := tx.Rollback(); err != nil {
			panic(err)
		}

		return nil, err
	}

	_, err = tx.Exec(`
		UPDATE volume
		SET status = "alloc"
		WHERE serial = ?
			AND status = "scratch"
	`, serial)

	if err != nil {
		if err := tx.Rollback(); err != nil {
			panic(err)
		}

		return nil, err
	}

	if err := tx.Commit(); err != nil {
		panic(err)
	}

	return &mtx.Volume{Serial: serial, Home: slot}, nil
}

func (inv *Inventory) Close() {
	inv.db.Close()
}

func (inv *Inventory) Audit(status *mtx.Status, libname string) error {
	// update volume locations
	for _, slot := range status.Slots {
		if slot.Vol != nil {
			// try to insert the row
			_, err := inv.db.Exec(`
				INSERT OR IGNORE INTO volume (serial, slot, status, library)
				VALUES (?, ?, ?, ?)
			`, slot.Vol.Serial, slot.Num, "scratch", libname)

			if err != nil {
				return err
			}

			// if it was ignored, make sure slot is updated... but EXPLICITLY
			// DO NOT RISK UPDATING STATUS TO SCRATCH FUCKHEAD!
			_, err = inv.db.Exec(`
				UPDATE volume
				SET slot = ?, library = ?
				WHERE serial = ?
			`, slot.Num, libname, slot.Vol.Serial)

			if err != nil {
				return err
			}
		}
	}

	return nil
}