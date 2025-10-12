# Scripts

Helper scripts to run Go tests and lightweight smoke probes.

* `unit.sh` – executes `go test ./...` with an isolated cache under the repo.
* `smoke.sh` – reads the active environment from `config/setting.ini` and curls the Token Exchange `/providers` endpoint.
