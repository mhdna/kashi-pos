DB = ./database

migrateup:
	migrate -path ./db/migrations -database "sqlite3://$(DB)" -verbose up

migrateup1:
	migrate -path ./db/migrations -database "sqlite3://$(DB)" -verbose up 1

migratedown:
	migrate -path ./db/migrations -database "sqlite3://$(DB)" -verbose down

migratedown1:
	migrate -path ./db/migrations -database "sqlite3://$(DB)" -verbose down 1

build-db:
	rm -f db/kashi.db
	@for f in $$(ls -1 db/migrations/*.up.sql | sort); do \
		sqlite3 db/kashi.db < "$$f"; \
	done
	sqlite3 db/kashi.db \
		"INSERT INTO roles (name, description) VALUES ('admin', 'Administrator');" \
		"INSERT INTO roles (name, description) VALUES ('cashier', 'Cashier');" \
		"INSERT INTO users (username, password_hash, role_id, active) VALUES ('admin', '8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918', 1, 1);" \
		"INSERT INTO users (username, password_hash, role_id, active) VALUES ('user', '04f8996da763b7a969b1028ee3007569eaf3a635486ddab211d512c85b9df8fb', 2, 1);" \
		"INSERT INTO settings (branch_name, api_key, printer_id, printer_size, theme) VALUES ('', '', '', '', 'light');"

sqlc:
	sqlc generate

test:
	go test -v -cover ./...

server:
	wails dev

.PHONY: migrateup migrateup1 migratedown migratedown1 build-db sqlc test server
