package helps

import (
	"github.com/fatih/color"
	"github.com/rodaine/table"
)

// Метод печати справки по использованию
func PrintHelpUsage() {
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	tbl := table.New(
		"OPERATION",
		"COMMAND EXAMPLE",
		"DESCRIPTION",
		"REQUIREMENTS AND NOTES",
	)
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	tbl.AddRow("CREATE NEW BUCKET", "-mb -bn <bucket name> -e <connection_name>", "create new bucket", "bucket can shouldn't exist")
	tbl.AddRow("DELETE EXISTS BUCKET", "-db -bn <bucket name> -e <connection_name>", "delete exists bucket", "bucket must be empty")
	tbl.AddRow("LIST BUCKETS", "-lb -o [table/json]", "list exists buckets", "-prefix <prefix> -maxentry <int>")
	tbl.AddRow("LIST BUCKET OBJECTS", "-lbo -bn <bucket name> -o [table/json] -e <connection_name>", "list bucket objects", "-sv for show versions, -maxentry <int>, -prefix <object prefix>")
	tbl.AddRow("LIST BUCKET OBJECTS UNIX STYLE", "-ls -bn <bucket name> -o [table/json] -e <connection_name>", "list bucket objects", "-sv for show versions, -maxentry <int>, -prefix <object prefix>")
	tbl.AddRow("LIST INCOMPLETE UPLOADS", "-liu -bn <bucket name> -o [table/json] -e <connection_name>", "list incomplete uploads", "-")
	tbl.AddRow("PUT BUCKET OBJECTS", "-po -path <path to file or folder> -bn <bucket name> -e <connection_name>", "upload file or dir to bucket", "if file exists create new version")
	tbl.AddRow("REMOVE BUCKET OBJECT", "-ro -bn <bucket name> -key <object key> -e <connection_name>", "remove object", "-vid <version id> for remove object version -f for force")
	tbl.AddRow("GET OBJECT METADATA", "-gos -bn <bucket name> -key <object key> -e <connection_name>", "get metadata for object", "file must be exists")
	tbl.AddRow("GET OBJECT", "-go -bn <bucket name> -key <object key> -e <connection_name>", "get object", "file must be exists")
	tbl.AddRow("REMOVE BUCKET OBJECTS FRON JSON FILE", `
		1. Get all bucket objects and versions
			- ./minioclient -e <connection_name> -bn <bucket_name> -lbo -sv -o json > <file_name>.json 
		2. Parse file and prepaire removabke array
			This filter by tag IsLatest == false
			- cat <file_name>.json | jq '[.[] | select(.IsLatest == false)]' > removable.json
		3. Init remove operation
			- ./minioclient -e <connection_name> -ro -bn <bucket name> -f -path ./removable.json
	`)
	tbl.AddRow("REMOVE BUCKET OBJECTS BY TAG", "-ro -tags '{\"IsLatest\":false,\"IsDeleteMarker\":true}' -bn <bucket name> -d", "remove object if all tags equivalents")
	tbl.AddRow("REMOVE BACCKET OBJECT BY ONE TAG NOT EQUIVALENTS", "-ro -not -tags '{\"IsLatest\":true,\"IsDeleteMarker\":false}' -bn <bucket name> -d -f", "remove object if one tags not equivalents")
	tbl.AddRow("REMOVE BACCKET OBJECT ALL NOT LAST VERSIONS AND FIX LEAK", "-ro -bn <bucket name> -d -bylastmodify -fixleak -leakcount <count> -indexpool <index pool name>", "remove all not last versions objects and fix leacks object for rgw index")
	tbl.AddRow("MIGRATE BUCKKET FROM SOURCE CLUSTER TO DST CLUSTER", "-e <source cluster from cfg> -ssl=[false|true] -migrate -destination <dst cluster from cfg> -bn <source bucket name> -dbn <dst bucket name> -d -maxentry <count threads>", "migrate bucket from source cluster to dst cluster")
	tbl.Print()
}
