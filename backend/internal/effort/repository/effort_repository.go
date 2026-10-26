// Package repository implementa con GORM la persistencia del módulo de registro de esfuerzo
// (Principio III).
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// EffortRepository implementa domain.EffortRepository con GORM sobre la tabla effort_entries.
type EffortRepository struct {
	db *gorm.DB
}

// NewEffortRepository crea el repositorio de esfuerzo sobre la conexión recibida.
func NewEffortRepository(db *gorm.DB) *EffortRepository {
	return &EffortRepository{db: db}
}

// entryAccessRow es la fila que lee FindAccessible: los identificadores del registro, de su historia,
// del proyecto de la historia y del autor, y el estado del proyecto con el alias project_status.
type entryAccessRow struct {
	EntryID       uuid.UUID
	StoryID       uuid.UUID
	ProjectID     uuid.UUID
	AuthorID      uuid.UUID
	ProjectStatus projectsdomain.ProjectStatus
}

// FindAccessible resuelve el acceso a un registro para RequireEffortEntryAccess con una sola consulta
// (research.md sección 3): une effort_entries con su historia, con la membresía vigente del usuario
// en project_members y con projects. Si no hay fila, el registro no existe o el usuario no es
// integrante vigente del proyecto, y ambos casos se reportan igual con ErrEntryNotFound, nunca
// gorm.ErrRecordNotFound ni (nil, nil). La autoría no se decide acá: AuthorID viaja en el resultado
// y la evalúa la escritura (FR-022).
func (r *EffortRepository) FindAccessible(ctx context.Context, entryID, userID uuid.UUID) (*domain.EntryAccess, error) {
	var row entryAccessRow
	err := r.db.WithContext(ctx).
		Table("effort_entries").
		Select(`effort_entries.id AS entry_id, effort_entries.story_id AS story_id,
			user_stories.project_id AS project_id, effort_entries.user_id AS author_id,
			projects.status AS project_status`).
		Joins("JOIN user_stories ON user_stories.id = effort_entries.story_id").
		Joins(`JOIN project_members ON project_members.project_id = user_stories.project_id
			AND project_members.user_id = ?`, userID).
		Joins("JOIN projects ON projects.id = user_stories.project_id").
		Where("effort_entries.id = ?", entryID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrEntryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolver el acceso al registro de esfuerzo: %w", err)
	}
	return &domain.EntryAccess{
		EntryID:         row.EntryID,
		StoryID:         row.StoryID,
		ProjectID:       row.ProjectID,
		AuthorID:        row.AuthorID,
		ProjectFinished: row.ProjectStatus == projectsdomain.ProjectStatusFinished,
	}, nil
}

// Create registra el alta de un registro de esfuerzo en una transacción, con el orden de bloqueos de
// research.md sección 10: (1) bloquea con FOR SHARE el proyecto de la historia y rechaza el proyecto
// Finalizado con ErrEffortReadOnly antes de mirar el plazo de gracia (FR-008); (2) valida la fecha
// contra hoy en la zona recibida y el inicio del proyecto (RN-05); (3) lee sin bloqueo los compromisos
// de la historia y resuelve el sprint con ResolveAnchor (FR-014, FR-016); (4) bloquea con FOR NO KEY
// UPDATE la fila del autor en users, suma sus horas de la fecha en todos los proyectos y aplica el
// tope diario (FR-005, RN-04); (5) inserta el registro, y (6) marca la historia con ever_had_effort
// sin tocar updated_at (RC-05). Devuelve el registro con su autor (solo id y nombre completo), su
// historia y su sprint, que es nil si quedó sin sprint.
func (r *EffortRepository) Create(ctx context.Context, storyID, authorID uuid.UUID, input domain.EntryInput, now time.Time, location *time.Location, grace time.Duration) (*domain.EntryView, error) {
	var view *domain.EntryView
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		story, err := lockStoryProject(tx, storyID)
		if err != nil {
			return err
		}
		if story.ProjectStatus == projectsdomain.ProjectStatusFinished {
			return domain.ErrEffortReadOnly
		}
		if err := domain.ValidateEntryDate(input.EntryDate, story.ProjectStartDate, calendarToday(now, location)); err != nil {
			return err
		}

		commitments, err := storyCommitments(tx, storyID)
		if err != nil {
			return err
		}
		sprintID, err := domain.ResolveAnchor(commitments.open(), commitments.lastClosed(), now, grace)
		if err != nil {
			return err
		}

		author, err := lockAuthor(tx, authorID)
		if err != nil {
			return err
		}
		logged, err := loggedHundredths(tx, authorID, input.EntryDate)
		if err != nil {
			return err
		}
		if err := domain.CheckDailyCap(logged, input.HoursHundredths); err != nil {
			return err
		}

		entry := domain.EffortEntry{
			ID:              uuid.New(),
			StoryID:         storyID,
			UserID:          authorID,
			SprintID:        sprintID,
			EntryDate:       input.EntryDate,
			Activity:        input.Activity,
			HoursHundredths: input.HoursHundredths,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("insertar el registro de esfuerzo: %w", err)
		}

		err = tx.
			Model(&backlogdomain.UserStory{}).
			Where("id = ? AND ever_had_effort = ?", storyID, false).
			UpdateColumn("ever_had_effort", true).Error
		if err != nil {
			return fmt.Errorf("marcar la historia con esfuerzo: %w", err)
		}

		view = &domain.EntryView{
			Entry:  entry,
			Author: *author,
			Story:  domain.StoryRef{ID: storyID, Title: story.Title},
			Sprint: commitments.sprint(sprintID),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// lockedStoryRow es la fila que lee lockStoryProject: el título de la historia y, por el JOIN a
// projects, el estado y la fecha de inicio de su proyecto con los alias project_status y
// project_start_date.
type lockedStoryRow struct {
	Title            string
	ProjectStatus    projectsdomain.ProjectStatus
	ProjectStartDate time.Time
}

// lockStoryProject lee, dentro de la transacción recibida, la historia unida a su proyecto con un
// bloqueo FOR SHARE solo sobre la fila de projects, para que el proyecto no pase a Finalizado hasta
// que termine la transacción (patrón lockWritableStoryProject de la 003). Si no hay fila, la historia
// no existe y devuelve ErrStoryNotFound.
func lockStoryProject(tx *gorm.DB, storyID uuid.UUID) (*lockedStoryRow, error) {
	var row lockedStoryRow
	err := tx.
		Table("user_stories").
		Select(`user_stories.title, projects.status AS project_status,
			projects.start_date AS project_start_date`).
		Joins("JOIN projects ON projects.id = user_stories.project_id").
		Where("user_stories.id = ?", storyID).
		Clauses(clause.Locking{Strength: "SHARE", Table: clause.Table{Name: "projects"}}).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, backlogdomain.ErrStoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("bloquear el proyecto de la historia: %w", err)
	}
	return &row, nil
}

// calendarToday devuelve la fecha de calendario de now en la zona recibida, como medianoche UTC, que
// es como se representan las columnas date (research.md sección 7).
func calendarToday(now time.Time, location *time.Location) time.Time {
	year, month, day := now.In(location).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// commitmentRow es un compromiso de la historia que lee storyCommitments: el sprint con su nombre,
// estado y cierre, y el resultado congelado, que es nil mientras el compromiso está abierto.
type commitmentRow struct {
	SprintID     uuid.UUID
	SprintName   string
	SprintStatus sprintsdomain.SprintStatus
	ClosedAt     *time.Time
	Outcome      *sprintsdomain.StoryOutcome
}

// commitments son los compromisos de una historia ordenados con los abiertos primero y luego por
// cierre más reciente, tal como los devuelve storyCommitments.
type commitments []commitmentRow

// storyCommitments lee sin bloqueo, en una sola sentencia y por lo tanto con una sola instantánea,
// todos los compromisos de la historia unidos a su sprint (research.md sección 10, paso 3). El orden
// por sprints.closed_at DESC NULLS FIRST deja primero el compromiso abierto, cuyo sprint no está
// cerrado, y después los congelados del cierre más reciente al más antiguo.
func storyCommitments(tx *gorm.DB, storyID uuid.UUID) (commitments, error) {
	rows := commitments{}
	err := tx.
		Table("sprint_stories").
		Select(`sprint_stories.sprint_id, sprints.name AS sprint_name, sprints.status AS sprint_status,
			sprints.closed_at, sprint_stories.outcome`).
		Joins("JOIN sprints ON sprints.id = sprint_stories.sprint_id").
		Where("sprint_stories.story_id = ?", storyID).
		Order("sprints.closed_at DESC NULLS FIRST, sprint_stories.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("leer los compromisos de la historia: %w", err)
	}
	return rows, nil
}

// open devuelve el compromiso abierto (outcome NULL) de la historia, o nil si no lo hay; la 004
// garantiza que hay como máximo uno.
func (c commitments) open() *domain.OpenCommitment {
	for _, row := range c {
		if row.Outcome == nil {
			return &domain.OpenCommitment{SprintID: row.SprintID}
		}
	}
	return nil
}

// lastClosed devuelve el compromiso congelado del sprint de cierre más reciente con su resultado, o
// nil si la historia nunca quedó en un sprint cerrado. Por el orden de storyCommitments es el primer
// compromiso con outcome; uno congelado sin closed_at no debería existir y se ignora.
func (c commitments) lastClosed() *domain.ClosedCommitment {
	for _, row := range c {
		if row.Outcome != nil && row.ClosedAt != nil {
			return &domain.ClosedCommitment{SprintID: row.SprintID, ClosedAt: *row.ClosedAt, Outcome: *row.Outcome}
		}
	}
	return nil
}

// sprint arma el sprint asociado de la respuesta a partir del compromiso leído del sprint resuelto,
// sin volver a consultar la base; devuelve nil si el registro quedó sin sprint.
func (c commitments) sprint(sprintID *uuid.UUID) *domain.AnchoredSprint {
	if sprintID == nil {
		return nil
	}
	for _, row := range c {
		if row.SprintID == *sprintID {
			return &domain.AnchoredSprint{ID: row.SprintID, Name: row.SprintName, Status: row.SprintStatus, ClosedAt: row.ClosedAt}
		}
	}
	return nil
}

// lockAuthor bloquea con FOR NO KEY UPDATE la fila del autor en users y lee solo su id y su nombre
// completo, nunca password_hash. El bloqueo serializa las escrituras de esfuerzo de esa persona para
// que el tope diario sea una invariante ante pedidos simultáneos, sin bloquear las inserciones que
// referencian al usuario con FOR KEY SHARE (research.md sección 9).
func lockAuthor(tx *gorm.DB, authorID uuid.UUID) (*domain.UserRef, error) {
	var author domain.UserRef
	err := tx.
		Table("users").
		Select("id, full_name").
		Where("id = ?", authorID).
		Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).
		Take(&author).Error
	if err != nil {
		return nil, fmt.Errorf("bloquear al autor del registro: %w", err)
	}
	return &author, nil
}

// loggedHundredths suma las centésimas de hora que la persona ya registró en la fecha, en todos los
// proyectos, incluidos los Finalizados y aquellos de los que fue quitada (RC-09).
func loggedHundredths(tx *gorm.DB, userID uuid.UUID, date time.Time) (int, error) {
	var total int
	err := tx.
		Model(&domain.EffortEntry{}).
		Select("COALESCE(SUM(hours_hundredths), 0)").
		Where("user_id = ? AND entry_date = ?", userID, date).
		Scan(&total).Error
	if err != nil {
		return 0, fmt.Errorf("sumar las horas del día: %w", err)
	}
	return total, nil
}

// Update modifica un registro propio en una transacción, con el orden de bloqueos de research.md
// sección 10: (1) bloquea con FOR SHARE el proyecto del registro y rechaza el proyecto Finalizado con
// ErrEffortReadOnly (FR-027); (2) exige que actorID sea el autor (FR-022); (3) bloquea con FOR UPDATE
// el registro releído con su sprint y rechaza el que ya es definitivo (FR-020); (4) valida la fecha
// resultante contra hoy en la zona recibida y el inicio del proyecto (RN-05); (5) bloquea con FOR NO
// KEY UPDATE la fila del autor en users y aplica el tope diario sobre la fecha resultante sin contar
// las horas vigentes del propio registro (FR-024), y (6) escribe solo la fecha, la actividad, las
// horas y updated_at: story_id, user_id y sprint_id nunca cambian (FR-023). Devuelve el registro
// actualizado con su autor, su historia y su sprint.
func (r *EffortRepository) Update(ctx context.Context, entryID, actorID uuid.UUID, patch domain.EntryPatch, now time.Time, location *time.Location, grace time.Duration) (*domain.EntryView, error) {
	var view *domain.EntryView
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		project, err := lockEntryProject(tx, entryID, actorID)
		if err != nil {
			return err
		}
		row, err := lockWritableEntry(tx, entryID, now, grace)
		if err != nil {
			return err
		}

		// Aplica el parche sobre una copia del registro releído y valida la fecha resultante.
		entry := row.EffortEntry
		if patch.EntryDate != nil {
			entry.EntryDate = *patch.EntryDate
		}
		if patch.Activity != nil {
			entry.Activity = *patch.Activity
		}
		if patch.HoursHundredths != nil {
			entry.HoursHundredths = *patch.HoursHundredths
		}
		if err := domain.ValidateEntryDate(entry.EntryDate, project.ProjectStartDate, calendarToday(now, location)); err != nil {
			return err
		}

		// Con el autor bloqueado, descuenta de la suma de la fecha resultante las horas vigentes del
		// propio registro, que solo están incluidas si la fecha no cambió, y aplica el tope.
		if _, err := lockAuthor(tx, actorID); err != nil {
			return err
		}
		logged, err := loggedHundredths(tx, actorID, entry.EntryDate)
		if err != nil {
			return err
		}
		if row.EntryDate.Equal(entry.EntryDate) {
			logged -= row.HoursHundredths
		}
		if err := domain.CheckDailyCap(logged, entry.HoursHundredths); err != nil {
			return err
		}

		err = tx.
			Model(&entry).
			Select("entry_date", "activity", "hours_hundredths", "updated_at").
			Updates(&entry).Error
		if err != nil {
			return fmt.Errorf("actualizar el registro de esfuerzo: %w", err)
		}

		row.EffortEntry = entry
		updated := row.view()
		view = &updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// Delete elimina un registro propio en una transacción, con el orden de bloqueos de research.md
// sección 10: proyecto FOR SHARE (Finalizado → ErrEffortReadOnly, FR-027), autoría
// (ErrNotEntryAuthor, FR-022) y registro FOR UPDATE releído con su sprint (ErrEffortFinalized,
// FR-020). No toma el bloqueo del autor, porque eliminar solo libera capacidad del tope diario, y no
// toca ever_had_effort: la historia sigue sin poder eliminarse aunque se quede sin registros (RC-05).
func (r *EffortRepository) Delete(ctx context.Context, entryID, actorID uuid.UUID, now time.Time, grace time.Duration) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockEntryProject(tx, entryID, actorID); err != nil {
			return err
		}
		if _, err := lockWritableEntry(tx, entryID, now, grace); err != nil {
			return err
		}
		if err := tx.Where("id = ?", entryID).Delete(&domain.EffortEntry{}).Error; err != nil {
			return fmt.Errorf("eliminar el registro de esfuerzo: %w", err)
		}
		return nil
	})
}

// entryProjectRow es la fila que lee lockEntryProject: el autor del registro y, por el JOIN a
// projects, el estado y la fecha de inicio del proyecto de su historia.
type entryProjectRow struct {
	AuthorID         uuid.UUID
	ProjectStatus    projectsdomain.ProjectStatus
	ProjectStartDate time.Time
}

// lockEntryProject lee, dentro de la transacción recibida, el registro unido a su historia y al
// proyecto con un bloqueo FOR SHARE solo sobre la fila de projects, para que el proyecto no pase a
// Finalizado hasta que termine la transacción. Aplica en este orden la regla de solo lectura
// (ErrEffortReadOnly) y la de autoría (ErrNotEntryAuthor); el autor se lee sin bloquear el registro
// porque user_id es inmutable. Si no hay fila, devuelve ErrEntryNotFound.
func lockEntryProject(tx *gorm.DB, entryID, actorID uuid.UUID) (*entryProjectRow, error) {
	var row entryProjectRow
	err := tx.
		Table("effort_entries").
		Select(`effort_entries.user_id AS author_id, projects.status AS project_status,
			projects.start_date AS project_start_date`).
		Joins("JOIN user_stories ON user_stories.id = effort_entries.story_id").
		Joins("JOIN projects ON projects.id = user_stories.project_id").
		Where("effort_entries.id = ?", entryID).
		Clauses(clause.Locking{Strength: "SHARE", Table: clause.Table{Name: "projects"}}).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrEntryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("bloquear el proyecto del registro de esfuerzo: %w", err)
	}
	if row.ProjectStatus == projectsdomain.ProjectStatusFinished {
		return nil, domain.ErrEffortReadOnly
	}
	if err := domain.EnsureAuthor(row.AuthorID, actorID); err != nil {
		return nil, err
	}
	return &row, nil
}

// lockWritableEntry relee el registro con un bloqueo FOR UPDATE solo sobre su fila de
// effort_entries, junto con su autor, su historia y el estado y el cierre de su sprint, y verifica
// con EnsureEntryWritable que todavía se pueda modificar o eliminar en now (FR-020, FR-021). Si el
// registro ya no existe, devuelve ErrEntryNotFound.
func lockWritableEntry(tx *gorm.DB, entryID uuid.UUID, now time.Time, grace time.Duration) (*entryRow, error) {
	var row entryRow
	err := entryViews(tx).
		Where("effort_entries.id = ?", entryID).
		Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "effort_entries"}}).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrEntryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("bloquear el registro de esfuerzo: %w", err)
	}
	if err := domain.EnsureEntryWritable(row.view().Sprint, now, grace); err != nil {
		return nil, err
	}
	return &row, nil
}

// entryRow es la fila plana que leen las consultas de listado: el registro, el nombre completo de su
// autor y el título de su historia, y los datos del sprint asociado, que son NULL cuando el registro
// no tiene sprint por el LEFT JOIN.
type entryRow struct {
	domain.EffortEntry
	AuthorFullName string
	StoryTitle     string
	SprintName     *string
	SprintStatus   *sprintsdomain.SprintStatus
	SprintClosedAt *time.Time
}

// view arma el EntryView de la fila; el sprint queda nil si el registro no tiene sprint.
func (row entryRow) view() domain.EntryView {
	view := domain.EntryView{
		Entry:  row.EffortEntry,
		Author: domain.UserRef{ID: row.UserID, FullName: row.AuthorFullName},
		Story:  domain.StoryRef{ID: row.StoryID, Title: row.StoryTitle},
	}
	if row.SprintID != nil && row.SprintName != nil && row.SprintStatus != nil {
		view.Sprint = &domain.AnchoredSprint{
			ID:       *row.SprintID,
			Name:     *row.SprintName,
			Status:   *row.SprintStatus,
			ClosedAt: row.SprintClosedAt,
		}
	}
	return view
}

// entryViews arma sobre db la consulta base de las filas entryRow: effort_entries unido a users (solo
// id y nombre completo, nunca password_hash), a user_stories para el título y con LEFT JOIN a sprints
// para el nombre, el estado y el cierre del sprint asociado.
func entryViews(db *gorm.DB) *gorm.DB {
	return db.
		Table("effort_entries").
		Select(`effort_entries.id, effort_entries.story_id, effort_entries.user_id,
			effort_entries.sprint_id, effort_entries.entry_date, effort_entries.activity,
			effort_entries.hours_hundredths, effort_entries.created_at, effort_entries.updated_at,
			users.full_name AS author_full_name, user_stories.title AS story_title,
			sprints.name AS sprint_name, sprints.status AS sprint_status,
			sprints.closed_at AS sprint_closed_at`).
		Joins("JOIN users ON users.id = effort_entries.user_id").
		Joins("JOIN user_stories ON user_stories.id = effort_entries.story_id").
		Joins("LEFT JOIN sprints ON sprints.id = effort_entries.sprint_id")
}

// ListForStory devuelve los registros de una historia en una sola consulta (research.md sección
// 11): effort_entries unido a users (solo id y nombre completo, nunca password_hash), a user_stories
// para el título y con LEFT JOIN a sprints para el nombre, el estado y el cierre del sprint asociado.
// El orden es el de FR-035: fecha descendente, creación ascendente e id ascendente como desempate.
// Una historia sin registros devuelve una lista vacía, nunca nil.
func (r *EffortRepository) ListForStory(ctx context.Context, storyID uuid.UUID) ([]domain.EntryView, error) {
	rows := []entryRow{}
	err := entryViews(r.db.WithContext(ctx)).
		Where("effort_entries.story_id = ?", storyID).
		Order("effort_entries.entry_date DESC, effort_entries.created_at ASC, effort_entries.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listar el esfuerzo de la historia: %w", err)
	}

	views := make([]domain.EntryView, 0, len(rows))
	for _, row := range rows {
		views = append(views, row.view())
	}
	return views, nil
}

// ListForProject devuelve los registros de un proyecto según el filtro en una sola consulta
// (research.md sección 11): la misma base que ListForStory, acotada por user_stories.project_id, con
// los filtros conjuntivos (FR-037). El sprint se filtra sobre effort_entries.sprint_id, el sprint
// asociado al registro y no el actual de la historia (FR-039), y WithoutSprint pide sprint_id IS NULL.
// Los extremos del rango de fechas son inclusivos (FR-038). El orden es el de FR-045, igual al de la
// historia. Sin coincidencias devuelve una lista vacía, nunca nil. El filtro llega ya verificado por
// ValidateFilter.
func (r *EffortRepository) ListForProject(ctx context.Context, projectID uuid.UUID, filter domain.EntryFilter) ([]domain.EntryView, error) {
	query := entryViews(r.db.WithContext(ctx)).
		Where("user_stories.project_id = ?", projectID)

	// Agrega solo las condiciones de los filtros presentes; GORM las combina con AND.
	if filter.UserID != nil {
		query = query.Where("effort_entries.user_id = ?", *filter.UserID)
	}
	if filter.StoryID != nil {
		query = query.Where("effort_entries.story_id = ?", *filter.StoryID)
	}
	switch {
	case filter.WithoutSprint:
		query = query.Where("effort_entries.sprint_id IS NULL")
	case filter.SprintID != nil:
		query = query.Where("effort_entries.sprint_id = ?", *filter.SprintID)
	}
	if filter.From != nil {
		query = query.Where("effort_entries.entry_date >= ?", *filter.From)
	}
	if filter.To != nil {
		query = query.Where("effort_entries.entry_date <= ?", *filter.To)
	}

	rows := []entryRow{}
	err := query.
		Order("effort_entries.entry_date DESC, effort_entries.created_at ASC, effort_entries.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listar el esfuerzo del proyecto: %w", err)
	}

	views := make([]domain.EntryView, 0, len(rows))
	for _, row := range rows {
		views = append(views, row.view())
	}
	return views, nil
}

// ValidateFilter verifica que los filtros por integrante, historia y sprint correspondan al proyecto
// (FR-044), en el orden userId, storyId, sprintId, y devuelve InvalidFilterError con el primer
// parámetro que no corresponde. El integrante es válido si es integrante vigente o si es autor de
// algún registro del proyecto, aunque ya no lo integre (FR-040); la historia y el sprint, si son del
// proyecto. Un identificador inexistente y uno de otro proyecto dan el mismo error, sin distinguirse.
// El valor "sin sprint" y las fechas no se verifican acá.
func (r *EffortRepository) ValidateFilter(ctx context.Context, projectID uuid.UUID, filter domain.EntryFilter) error {
	db := r.db.WithContext(ctx)

	if filter.UserID != nil {
		query := db.
			Table("users").
			Where("users.id = ?", *filter.UserID).
			Where(`EXISTS (SELECT 1 FROM project_members
					WHERE project_members.project_id = ? AND project_members.user_id = users.id)
				OR EXISTS (SELECT 1 FROM effort_entries
					JOIN user_stories ON user_stories.id = effort_entries.story_id
					WHERE user_stories.project_id = ? AND effort_entries.user_id = users.id)`,
				projectID, projectID)
		if err := requireFilterMatch(query, "userId"); err != nil {
			return err
		}
	}
	if filter.StoryID != nil {
		query := db.
			Table("user_stories").
			Where("id = ? AND project_id = ?", *filter.StoryID, projectID)
		if err := requireFilterMatch(query, "storyId"); err != nil {
			return err
		}
	}
	if filter.SprintID != nil && !filter.WithoutSprint {
		query := db.
			Table("sprints").
			Where("id = ? AND project_id = ?", *filter.SprintID, projectID)
		if err := requireFilterMatch(query, "sprintId"); err != nil {
			return err
		}
	}
	return nil
}

// requireFilterMatch cuenta las filas de la consulta recibida y devuelve InvalidFilterError con el
// parámetro field si no hay ninguna.
func requireFilterMatch(query *gorm.DB, field string) error {
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return fmt.Errorf("verificar el filtro %s: %w", field, err)
	}
	if count == 0 {
		return &domain.InvalidFilterError{Field: field}
	}
	return nil
}

// HasProjectEffort informa si el proyecto tiene al menos un registro de esfuerzo, sin filtros, para
// distinguir el proyecto sin esfuerzo (no_effort) de los filtros sin resultados (no_matches).
func (r *EffortRepository) HasProjectEffort(ctx context.Context, projectID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.WithContext(ctx).
		Raw(`SELECT EXISTS (SELECT 1 FROM effort_entries
			JOIN user_stories ON user_stories.id = effort_entries.story_id
			WHERE user_stories.project_id = ?)`, projectID).
		Scan(&exists).Error
	if err != nil {
		return false, fmt.Errorf("verificar si el proyecto tiene esfuerzo: %w", err)
	}
	return exists, nil
}

// ComparisonRows devuelve el factor de horas por Story Point vigente del proyecto y las filas de la
// comparación (FR-046 a FR-053): las historias del proyecto, o solo storyID si no es nil, con sus
// Story Points y la suma de sus horas reales por LEFT JOIN agregado, 0 si no tienen registros. El
// orden es el del listado del backlog de la 003: prioridad, created_at e id. El acceso al proyecto y
// a la historia ya lo resolvieron los middlewares de delivery.
func (r *EffortRepository) ComparisonRows(ctx context.Context, projectID uuid.UUID, storyID *uuid.UUID) (int, []domain.ComparisonRow, error) {
	db := r.db.WithContext(ctx)

	// El factor se lee siempre de projects, sin copiarlo, para que un cambio de factor recalcule las
	// estimadas en la consulta siguiente (FR-050).
	var factorHundredths int
	err := db.
		Table("projects").
		Select("hours_per_story_point_hundredths").
		Where("id = ?", projectID).
		Take(&factorHundredths).Error
	if err != nil {
		return 0, nil, fmt.Errorf("leer el factor de horas por Story Point del proyecto: %w", err)
	}

	// Las horas reales se agregan con LEFT JOIN y COALESCE para que la historia sin registros aparezca
	// con 0 y no desaparezca de la comparación.
	query := db.
		Table("user_stories").
		Select(`user_stories.id AS story_id, user_stories.title AS title,
			user_stories.story_points AS story_points,
			COALESCE(SUM(effort_entries.hours_hundredths), 0) AS actual_hundredths`).
		Joins("LEFT JOIN effort_entries ON effort_entries.story_id = user_stories.id").
		Where("user_stories.project_id = ?", projectID)
	if storyID != nil {
		query = query.Where("user_stories.id = ?", *storyID)
	}

	rows := []domain.ComparisonRow{}
	err = query.
		Group("user_stories.id").
		Order("CASE user_stories.priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, " +
			"user_stories.created_at ASC, user_stories.id ASC").
		Scan(&rows).Error
	if err != nil {
		return 0, nil, fmt.Errorf("listar las filas de la comparación de esfuerzo: %w", err)
	}
	return factorHundredths, rows, nil
}
