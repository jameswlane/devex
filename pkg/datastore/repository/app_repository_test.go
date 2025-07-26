package repository_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/jameswlane/devex/pkg/datastore"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/jameswlane/devex/pkg/datastore/repository"
)

type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) Conn() *sql.DB {
	return nil // Mock implementation, not used in tests
}

func (m *MockDatabase) Exec(query string, args ...interface{}) error {
	call := m.Called(append([]interface{}{query}, args...)...)
	return call.Error(0)
}

func (m *MockDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	call := m.Called(append([]interface{}{query}, args...)...)
	if call.Get(0) == nil {
		// Simulate an error by returning a row that returns an error on Scan
		return &sql.Row{} // Will cause Scan to return sql.ErrNoRows
	}
	return call.Get(0).(*sql.Row)
}

func (m *MockDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	call := m.Called(append([]interface{}{query}, args...)...)
	if call.Get(0) == nil {
		return nil, call.Error(1)
	}
	return call.Get(0).(*sql.Rows), call.Error(1)
}

func (m *MockDatabase) Close() error {
	return nil // Mock implementation for Close
}

type MockRows struct {
	mock.Mock
}

func (m *MockRows) Next() bool {
	return m.Called().Bool(0)
}

func (m *MockRows) Scan(dest ...interface{}) error {
	call := m.Called(dest)
	return call.Error(0)
}

func (m *MockRows) Close() error {
	return m.Called().Error(0)
}

func TestAddApp_Success(t *testing.T) {
	db := datastore.NewInMemorySQLite()
	defer db.Close()

	repo := repository.NewAppRepository(db)

	// Add an app
	err := repo.AddApp("testApp")
	assert.NoError(t, err)

	// Verify using direct SQL query
	var count int
	row := db.Conn().QueryRow("SELECT COUNT(*) FROM installed_apps WHERE app_name = ?", "testApp")
	err = row.Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count, "App should be added to the database")
}

func TestAddApp_Failure(t *testing.T) {
	db := new(MockDatabase)
	repo := repository.NewAppRepository(db)

	db.On("Exec", "INSERT INTO installed_apps (app_name) VALUES (?)", "testApp").
		Return(errors.New("insert error"))

	err := repo.AddApp("testApp")
	assert.Error(t, err)
	db.AssertExpectations(t)
}

func TestGetApp_Exists(t *testing.T) {
	db := datastore.NewInMemorySQLite()
	defer db.Close()

	err := db.Exec("INSERT INTO installed_apps (app_name) VALUES (?)", "testApp")
	assert.NoError(t, err)

	repo := repository.NewAppRepository(db)

	// Check if the app exists
	exists, err := repo.GetApp("testApp")
	assert.NoError(t, err)
	assert.True(t, exists)

	// Check a non-existent app
	exists, err = repo.GetApp("nonExistentApp")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestGetApp_NotExists(t *testing.T) {
	db := new(MockDatabase)
	repo := repository.NewAppRepository(db)

	mockRow := new(MockRow)
	mockRow.On("Scan", mock.Anything).Return(sql.ErrNoRows)

	db.On("QueryRow", "SELECT 1 FROM installed_apps WHERE app_name = ? LIMIT 1", "testApp").
		Return(mockRow)

	exists, err := repo.GetApp("testApp")
	assert.NoError(t, err)
	assert.False(t, exists)
	db.AssertExpectations(t)
}

func TestGetApp_QueryError(t *testing.T) {
	db := new(MockDatabase)
	repo := repository.NewAppRepository(db)

	db.On("QueryRow", "SELECT 1 FROM installed_apps WHERE app_name = ? LIMIT 1", "testApp").
		Return(nil)

	exists, err := repo.GetApp("testApp")
	assert.Error(t, err)
	assert.False(t, exists)
	db.AssertExpectations(t)
}

func TestRemoveApp_Success(t *testing.T) {
	db := datastore.NewInMemorySQLite()
	defer db.Close()

	err := db.Exec("INSERT INTO installed_apps (app_name) VALUES (?)", "testApp")
	assert.NoError(t, err)

	repo := repository.NewAppRepository(db)

	// Remove the app
	err = repo.RemoveApp("testApp")
	assert.NoError(t, err)

	// Verify using direct SQL query
	var count int
	row := db.Conn().QueryRow("SELECT COUNT(*) FROM installed_apps WHERE app_name = ?", "testApp")
	err = row.Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count, "App should be removed from the database")
}

func TestRemoveApp_Failure(t *testing.T) {
	db := new(MockDatabase)
	repo := repository.NewAppRepository(db)

	db.On("Exec", "DELETE FROM installed_apps WHERE app_name = ?", "testApp").
		Return(errors.New("delete error"))

	err := repo.RemoveApp("testApp")
	assert.Error(t, err)
	db.AssertExpectations(t)
}

func TestListAllApps_Success(t *testing.T) {
	db := new(MockDatabase)
	repo := repository.NewAppRepository(db)

	mockRows := new(MockRows)
	mockRows.On("Next").Return(true).Once()  // Simulate one row
	mockRows.On("Next").Return(false).Once() // End of rows
	mockRows.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		*(args.Get(0).(*string)) = "testApp"
	}).Return(nil)

	db.On("Query", "SELECT app_name FROM installed_apps").Return(mockRows, nil)

	apps, err := repo.ListAllApps()
	assert.NoError(t, err)
	assert.Equal(t, []string{"testApp"}, apps)
	db.AssertExpectations(t)
}

func TestListAllApps_QueryError(t *testing.T) {
	db := new(MockDatabase)
	repo := repository.NewAppRepository(db)

	db.On("Query", "SELECT app_name FROM installed_apps").Return(nil, errors.New("query error"))

	apps, err := repo.ListAllApps()
	assert.Error(t, err) // Ensure error is returned
	assert.Nil(t, apps)
	db.AssertExpectations(t)
}
