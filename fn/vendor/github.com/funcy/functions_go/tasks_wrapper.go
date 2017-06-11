package functions

type TasksWrapper struct {

	Tasks []Task `json:"tasks,omitempty"`

	// Used to paginate results. If this is returned, pass it into the same query again to get more results.
	Cursor string `json:"cursor,omitempty"`

	Error_ ErrorBody `json:"error,omitempty"`
}
