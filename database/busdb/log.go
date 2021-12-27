package busdb

import (
	"context"
	"errors"
	"github.com/SecurityBrewery/catalyst/bus"
	"strings"

	"github.com/arangodb/go-driver"

	"github.com/SecurityBrewery/catalyst/generated/models"
	"github.com/SecurityBrewery/catalyst/time"
)

const LogCollectionName = "logs"

func (db *BusDatabase) LogCreate(ctx context.Context, logType, reference, message string) (*models.LogEntry, error) {
	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, errors.New("no user in context")
	}

	logentry := &models.LogEntry{
		Type:      logType,
		Reference: reference,
		Created:   time.Now().UTC(),
		Creator:   user.ID,
		Message:   message,
	}

	doc := models.LogEntry{}
	_, err := db.logCollection.CreateDocument(driver.WithReturnNew(ctx, &doc), logentry)
	if err != nil {
		return nil, err
	}

	return &doc, nil
}

func (db *BusDatabase) LogBatchCreate(ctx context.Context, logentries []*models.LogEntry) error {
	var ids []driver.DocumentID
	for _, entry := range logentries {
		if strings.HasPrefix(entry.Reference, "tickets/") {
			ids = append(ids, driver.DocumentID(entry.Reference))
		}
	}
	if ids != nil {
		go db.bus.PublishDatabaseUpdate(ids, bus.DatabaseEntryCreated)
	}

	_, errs, err := db.logCollection.CreateDocuments(ctx, logentries)
	if err != nil {
		return err
	}
	err = errs.FirstNonNil()
	if err != nil {
		return err
	}

	return nil
}

func (db *BusDatabase) LogList(ctx context.Context, reference string) ([]*models.LogEntry, error) {
	query := "FOR d IN @@collection FILTER d.reference == @reference SORT d.created DESC RETURN d"
	cursor, err := db.internal.Query(ctx, query, map[string]interface{}{
		"@collection": LogCollectionName,
		"reference":   reference,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close()
	var docs []*models.LogEntry
	for {
		var doc models.LogEntry
		_, err := cursor.ReadDocument(ctx, &doc)
		if driver.IsNoMoreDocuments(err) {
			break
		} else if err != nil {
			return nil, err
		}
		docs = append(docs, &doc)
	}

	return docs, err
}
