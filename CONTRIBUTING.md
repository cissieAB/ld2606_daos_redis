# Contributing to ld2606_daos_redis

## Repository Structure

This is a **multi-component** repository. Each component is independent but shares the traffic simulator.

```
ld2606_daos_redis/
├── backend/              # Go server
├── traffic-simulator/    # Shared mock generator
└── daos-client/          # DAOS client
```

## Development Guidelines

### 1. Component Independence
- Each component has its own README and setup
- Components communicate through Redis
- Changes to one component shouldn't break others

### 2. Shared Components
- **traffic-simulator/** is shared by both backend and daos-client
- Changes to simulator should be coordinated with both teams
- Keep backward compatibility when modifying simulator

### 3. Documentation
- Update relevant README when making changes
- **Avoid duplication**: Link to other READMEs instead of copying
- Keep main [README.md](README.md) as the central navigation point

### 4. Testing
- Test your component with the traffic simulator
- Ensure Redis integration works
- Document any new environment variables

## Workflow for Backend Team

```bash
cd backend/
# ... make changes ...
./setup.sh  # Test setup
go run .    # Test functionality

# Test with simulator
cd ../traffic-simulator
python simulator.py --redis-host localhost
```

## Workflow for DAOS Team

```bash
cd daos-client/
# ... make changes ...

# Test with simulator
cd ../traffic-simulator
python simulator.py --redis-host localhost
```

## Git Workflow

### Branch Naming
- `backend/feature-name` - Backend changes
- `simulator/feature-name` - Simulator changes  
- `daos/feature-name` - DAOS client changes
- `docs/topic` - Documentation only

### Commit Messages
```
component: brief description

Detailed explanation if needed

Relates to: #issue-number
```

Examples:
- `backend: add health check endpoint`
- `simulator: fix UDP typo in packet generation`
- `daos: implement initial Redis reader`
- `docs: update architecture diagram`

### Pull Requests
- Clearly indicate which component(s) are affected
- Tag relevant team members for review

## Questions?

- Check component-specific READMEs first
- Ask in team chat/issues
- Update documentation when you find answers

## Links

- [Main README](README.md)
- [Backend README](backend/README.md)
- [Simulator README](traffic-simulator/README.md)
- [DAOS Client README](daos-client/README.md)
