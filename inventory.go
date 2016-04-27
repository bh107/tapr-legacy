package tapr

import (
	"bytes"
	"database/sql"

	// import for side effects (load the sqlite3 driver)
	_ "github.com/mattn/go-sqlite3"
)

type Inventory struct {
	db *sql.DB
}

func (inv *Inventory) slot(vol *Volume) (*Slot, error) {
	row := inv.db.QueryRow(`SELECT slot, library FROM volume WHERE serial = ?`, vol.Serial)

	var slotnum int
	var libname string

	if err := row.Scan(&slotnum, &libname); err != nil {
		return nil, err
	}

	return &Slot{
		ID:      slotnum,
		Volume:  vol,
		Libname: libname,
	}, nil
}

func (inv *Inventory) volumes(libname string) ([]*Volume, error) {
	rows, err := inv.db.Query(`
		SELECT serial
		FROM volume
		WHERE library = ?
	`, libname)
	if err != nil {
		return nil, err
	}

	var vols []*Volume
	for rows.Next() {
		var serial string
		if err := rows.Scan(&serial); err != nil {
			return nil, err
		}

		vols = append(vols, &Volume{Serial: serial})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return vols, nil
}

func (inv *Inventory) scratch(libname string) (*Volume, error) {
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

	return &Volume{Serial: serial, Slot: slot}, nil
}

func (inv *Inventory) audit(b []byte, libname string) (*mtxStatus, error) {
	status, err := mtxParseStatus(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	// update volume locations
	for _, slot := range status.slots {
		if slot.vol != nil {
			// try to insert the row
			_, err := inv.db.Exec(`
				INSERT OR IGNORE INTO volume (serial, slot, status, library)
				VALUES (?, ?, ?, ?)
			`, slot.vol.Serial, slot.id, "scratch", libname)

			if err != nil {
				return nil, err
			}

			// if it was ignored, make sure slot is updated... but EXPLICITLY
			// DO NOT RISK UPDATING STATUS TO SCRATCH FUCKHEAD!
			_, err = inv.db.Exec(`
				UPDATE volume
				SET slot = ?, library = ?
				WHERE serial = ?
			`, slot.id, libname, slot.vol.Serial)

			if err != nil {
				return nil, err
			}
		}
	}

	return status, nil
}
