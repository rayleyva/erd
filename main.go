package main

import (
	"os"
	"text/template"
	"log"
	"io/ioutil"
)

type Relation struct {
	TableName  string
	ColumnName string
}

type Column struct {
	Name     string
	Relation *Relation
}

type Table struct {
	Name    string
	Columns []Column
}

func (t Table) ColumnsWithRelation() []Column {
	ret := make([]Column, 0)
	for _, c := range t.Columns {
		if c.Relation != nil {
			ret = append(ret, c)
		}
	}
	return ret
}

func ReadStdin() string {
	buf, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err.Error())
	}
	return string(buf)
}

func main() {
	//const s string = `
	//this {
	//id
	//that -> that.id
	//}
	//
	//that {
	//id
	//name
	//}
	//`  // 解析対象文字列
	c := ReadStdin()

	parser := &Parser{Buffer: c}  // 解析対象文字の設定
	parser.Init()                 // parser初期化
	err := parser.Parse()         // 解析
	if err != nil {
		log.Fatal(err.Error())
	} else {
		parser.Execute()          // アクション処理
	}

	////{{.Name}}[label="<B>{{.Name}}</B>{{range .Columns}}|<{{.Name}}>{{.Name}}{{end}}"];

	tmpl, err := template.New("test").Parse(`
digraph er {
	graph [rankdir=LR];
	ranksep="1.2";
	overlap=false;
	splines=true;
	sep="+30,30";
	node [shape=plaintext];
	edge [fontsize=7];
{{range .Tables}}
{{.Name}}[label=<
<TABLE STYLE="RADIAL" BORDER="1" CELLBORDER="0" CELLSPACING="1" ROWS="*">
  <TR><TD><B>{{.Name}}</B></TD></TR>
  {{range .Columns}}
    <TR><TD PORT="{{.Name}}">{{.Name}}</TD></TR>
  {{end}}
</TABLE>
>];
{{end}}

{{range $table := .Tables}}
{{range $column := $table.ColumnsWithRelation}}
{{$table.Name}}:{{$column.Name}} -> {{$column.Relation.TableName}}:{{$column.Relation.ColumnName}};
{{end}}
{{end}}
}
		`)
	if err != nil {
		log.Fatalf(err.Error())
	}

	err = tmpl.Execute(os.Stdout, parser)
	if err != nil {
		log.Fatalf(err.Error())
	}
}