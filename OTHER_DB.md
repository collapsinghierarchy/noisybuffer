
## 🛠️ Using a database other than Postgres

NoisyBuffer’s Go core depends only on the `store.Store` interface.  
Swap in **any** storage backend—MySQL, SQLite, MongoDB, DynamoDB, or an
in‑memory map—by implementing six small methods.

---

### 1 · Skeleton adapter

```go
package mystore

import (
	"context"
	"database/sql"        // or your driver
	"github.com/google/uuid"

	"github.com/collapsinghierarchy/noisybuffer/model"
	"github.com/collapsinghierarchy/noisybuffer/store"
)

type myStore struct{ db *sql.DB }           // use your own handle

func New(db *sql.DB) store.Store { return &myStore{db: db} }

// -------- submissions ----------------------------------------------
func (m *myStore) InsertSubmission(ctx context.Context, s *model.Submission) error {
	// INSERT INTO submissions (…)  OR  collection.InsertOne(…)
	return nil
}

func (m *myStore) StreamSubmissions(
	ctx context.Context, appID uuid.UUID,
	fn func(*model.Submission) error,
) error {
	// SELECT … ORDER BY ts ASC; for each row call fn(&sub)
	return nil
}

// -------- key registry ---------------------------------------------
func (m *myStore) AppExists(ctx context.Context, id uuid.UUID) (bool, error) {
	return false, nil
}

func (m *myStore) RegisterKey(ctx context.Context, appID uuid.UUID,
	kid uint8, pub []byte) error {
	return nil
}

func (m *myStore) GetKey(ctx context.Context, appID uuid.UUID) (uint8, []byte, error) {
	return 0, nil, nil
}
```

---

### 2 · Use it in `main.go`

```go
st  := mystore.New(myDB)       // <‑‑ custom adapter
svc := service.New(st, 64*1024)
api := handler.SetupNBRoutes(svc)
```

No other code changes.

---

### 3 · Schema hints

| Concept      | Minimum fields (SQL) | Example in a NoSQL store |
|--------------|----------------------|--------------------------|
| **apps**     | `id UUID`    `kid SMALLINT`    `pub BYTEA` | `{_id:"uuid", kid:0, pub:<bytes>}` |
| **blobs**    | `id UUID`    `app_id UUID`    `kid SMALLINT`    `ts TIMESTAMPTZ`    `blob BYTEA` | `{_id:"uuid", app:"uuid", kid:0, ts:"2025‑07‑13T…", blob:<bytes>}` |

Indexes: `(app_id, ts)` is usually enough.

---

