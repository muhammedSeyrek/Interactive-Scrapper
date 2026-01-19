# Initial commit.

## If you here in linux nothing doing but you using other os, so start docker program in the beginning.

# Run
docker-compose up -d --build

# View logs
docker-compose logs -f app

# Stop
docker-compose down

# Stop and remove database (fresh start)
docker-compose down -v