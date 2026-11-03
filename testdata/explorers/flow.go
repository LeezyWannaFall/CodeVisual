package explorers

type UserRepository interface{ Fetch(string) string }
type MemoryRepository struct{}

func (*MemoryRepository) Fetch(id string) string { return normalize(id) }
func normalize(id string) string                 { return id }

type UserService struct{ repo UserRepository }

func (s *UserService) User(id string) string { return s.repo.Fetch(id) }

var userService = &UserService{repo: &MemoryRepository{}}
