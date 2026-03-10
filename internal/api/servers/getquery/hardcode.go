package getquery

import "github.com/gameap/gameap/pkg/quercon/query"

var queryProtocolsByEngine = map[string]query.Protocol{
	"arma":        query.ProtocolGameSpy2,
	"arma3":       query.ProtocolSource,
	"goldsource":  query.ProtocolSource,
	"goldsrc":     query.ProtocolSource,
	"minecraft":   query.ProtocolMinecraft,
	"minecraftbr": query.ProtocolRakNet,
	"q2":          query.ProtocolQuake2,
	"q3":          query.ProtocolQuake3,
	"samp":        query.ProtocolSAMP,
	"source":      query.ProtocolSource,
}

func getQueryProtocolByEngine(engine string) (query.Protocol, bool) {
	protocol, ok := queryProtocolsByEngine[engine]

	return protocol, ok
}
