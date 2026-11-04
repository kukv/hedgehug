# the findings and the schema checks of the old doctor: schemas and
# maintenance both use them, so they go down to the base
from internal/app/usecase/maintenance
to internal/app/usecase usecase
file finding.go finding.go
file schema.go schema_findings.go
rename Error SeverityError
rename Warning SeverityWarning
rename Info SeverityInfo
rename Sort SortFindings
rename Failing FindingsFail
rename Schema CheckSchema
rename SchemaFile CheckSchemaFile
rename brokenHint BrokenSchemaHint
rename taskWhere TaskWhere
