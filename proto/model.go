package proto

import (
	"errors"
	"fmt"
	"github.com/niiigoo/hawk/proto/io"
	errors2 "github.com/pkg/errors"
	"strings"
)

type Type int

const (
	TypeScalar Type = iota + 1
	TypeMap
	TypeEnum
	TypeMessage
	TypeOneOf
)

type Location string

const (
	LocationPath  Location = "path"
	LocationQuery          = "query"
	LocationBody           = "body"
)

type Definition struct {
	syntax      string
	pack        string
	services    []*io.Service
	imports     []string
	enums       []*io.Enum
	enumsMap    map[string]*io.Enum
	messages    []*io.Message
	messagesMap map[string]*io.Message

	Services []*Service
}

func (d Definition) Package() string {
	return d.pack
}

type Service struct {
	*io.Service
	Name       string
	HttpPrefix string
	Compressed *bool
	WSPath     string
	WSDefault  *bool
	Methods    []*Method
}

type Method struct {
	*io.Method
	Type           Type
	Name           string
	Request        string
	RequestStream  bool
	Response       string
	ResponseStream bool
	HttpBindings   []*OptionHttp
	Compressed     bool
	WebSocket      bool

	Parent *Service
}

type OptionHttp struct {
	Method       string
	PathRaw      string
	Body         string
	ResponseBody string

	Path   *io.Path
	Params []*Param

	Parent *Method
}

type Param struct {
	*io.Field
	Type        Type
	Location    Location
	OneOfFields map[string]*Param
}

func (d Definition) getType(field *io.Field) Type {
	if field.Type.Scalar > io.None {
		return TypeScalar
	} else if field.Type.Map != nil {
		return TypeMap
	} else if _, ok := d.messagesMap[field.Type.Reference]; ok {
		return TypeMessage
	} else if _, ok = d.enumsMap[field.Type.Reference]; ok {
		return TypeEnum
	}
	return 0
}

func (o *OptionHttp) GorillaMuxPath() string {
	var path string
	if o.Parent != nil && o.Parent.Parent != nil {
		path = strings.TrimSuffix(o.Parent.Parent.HttpPrefix, "/")
	}

	l := len(o.Path.Segments)
	for i, segment := range o.Path.Segments {
		path += "/"
		if segment.Literal != nil {
			path += *segment.Literal
		} else if segment.Wildcard != nil {
			if len(*segment.Wildcard) > 1 && i == l-1 {
				// TODO: Does gorilla support **?
				path += ".*"
			} else {
				path += ".*"
			}
		} else if segment.Variable != nil {
			path += "{" + segment.Variable.Field
			if segment.Variable.Pattern != nil {
				path += ":" + *segment.Variable.Pattern
			}
			path += "}"
		}
	}

	return path
}

func (m *Method) CheckParams(def *Definition) error {
	msg, ok := def.messagesMap[m.Request]
	if !ok {
		return errors.New("message `" + m.Request + "` not found")
	}
	fields := make(map[string]*Param)
	for _, f := range msg.Entries {
		if f.Field != nil {
			fields[f.Field.Name] = &Param{
				Field: f.Field,
				Type:  def.getType(f.Field),
			}
		} else if f.OneOf != nil {
			fields[f.OneOf.Name] = &Param{
				OneOfFields: map[string]*Param{},
				Field:       &io.Field{Name: f.OneOf.Name},
				Type:        TypeOneOf,
			}
			for _, entry := range f.OneOf.Entries {
				fields[f.OneOf.Name].OneOfFields[entry.Field.Name] = &Param{
					Field: entry.Field,
					Type:  def.getType(entry.Field),
				}
			}
		}
	}

	for _, binding := range m.HttpBindings {
		binding.Params = make([]*Param, 0)

		params := make(map[string]bool)
		for _, entry := range msg.Entries {
			if entry.Field != nil {
				params[entry.Field.Name] = false
			} else if entry.OneOf != nil {
				params[entry.OneOf.Name] = false
			}
		}

		for _, s := range binding.Path.Segments {
			if s.Variable != nil {
				if _, ok = fields[s.Variable.Field]; !ok {
					return errors.New(fmt.Sprintf("path parameter `%s` not found (method `%s`)", s.Variable.Field, m.Name))
				}
				fields[s.Variable.Field].Location = LocationPath
				binding.Params = append(binding.Params, fields[s.Variable.Field])
				params[s.Variable.Field] = true
			}
		}

		if binding.Body != "*" {
			if binding.Body != "" {
				if f, ok := fields[binding.Body]; ok {
					f.Location = LocationBody
					binding.Params = append(binding.Params, f)
					params[binding.Body] = true
				} else {
					return errors.New(fmt.Sprintf("body field `%s` not found (method `%s`)", binding.Body, m.Name))
				}
			}

			for name, handled := range params {
				if handled {
					continue
				}

				fields[name].Location = LocationQuery
				binding.Params = append(binding.Params, fields[name])
			}
		}
	}

	return nil
}

func (s *Service) CheckParams(def *Definition) error {
	for _, method := range s.Methods {
		err := method.CheckParams(def)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) CompressionUsed() bool {
	for _, m := range s.Methods {
		if m.Compressed {
			return true
		}
	}
	return false
}

func (d Definition) methodFromProto(s *Service, method *io.Method) (*Method, error) {
	if method.Request == nil || method.Response == nil {
		return nil, errors.New("invalid method definition (`" + method.Name + "`)")
	}

	m := &Method{
		Method:         method,
		Name:           method.Name,
		Request:        method.Request.Reference,
		RequestStream:  method.StreamingRequest,
		Response:       method.Response.Reference,
		ResponseStream: method.StreamingResponse,
		HttpBindings:   make([]*OptionHttp, 0),
		Parent:         s,
	}
	if s.Compressed != nil {
		m.Compressed = *s.Compressed
	}
	if s.WSDefault != nil {
		m.WebSocket = *s.WSDefault
	}

	for _, option := range method.Options {
		if option.Name == "google.api.http" {
			if method.StreamingRequest || method.StreamingResponse {
				return nil, errors.New("streaming methods cannot have `google.api.http` option (method `" + method.Name + "`)")
			}

			if option.Value == nil || option.Value.Map == nil {
				return nil, errors.New("invalid value provided for `google.api.http` (method `" + method.Name + "`)")
			}
			err := m.parseBinding(option.Value.Map.Entries)
			if err != nil {
				return nil, err
			}
		} else if option.Name == "httpCompress" {
			if option.Value == nil || option.Value.Bool == nil {
				return nil, errors.New("invalid value provided for `httpCompress` (method `" + method.Name + "`)")
			}
			m.Compressed = bool(*option.Value.Bool)
		} else if option.Name == "webSocket" {
			if option.Value == nil || option.Value.Bool == nil {
				return nil, errors.New("invalid value provided for `webSocket` (method `" + method.Name + "`)")
			}
			m.WebSocket = bool(*option.Value.Bool)
		}
	}

	return m, nil
}

func (m *Method) parseBinding(data []*io.MapEntry) error {
	b := &OptionHttp{
		Parent: m,
	}
	for _, entry := range data {
		if entry.Key == nil || entry.Key.Reference == nil {
			return errors.New("invalid key of `google.api.http` (method `" + m.Name + "`)")
		}
		switch *entry.Key.Reference {
		case "get":
			fallthrough
		case "put":
			fallthrough
		case "post":
			fallthrough
		case "patch":
			fallthrough
		case "delete":
			b.Method = *entry.Key.Reference
			if entry.Value == nil || entry.Value.String == nil {
				return errors.New("invalid value provided of `" + *entry.Key.Reference + "` (method `" + m.Name + "`)")
			}
			b.PathRaw = *entry.Value.String
		case "body":
			if entry.Value == nil || entry.Value.String == nil {
				return errors.New("invalid value provided of `" + *entry.Key.Reference + "` (method `" + m.Name + "`)")
			}
			b.Body = *entry.Value.String
		case "response_body":
			if entry.Value == nil || entry.Value.String == nil {
				return errors.New("invalid value provided of `" + *entry.Key.Reference + "` (method `" + m.Name + "`)")
			}
			b.ResponseBody = *entry.Value.String
		case "custom":
			if entry.Value == nil || entry.Value.Map == nil {
				return errors.New("invalid value provided of `" + *entry.Key.Reference + "` (method `" + m.Name + "`)")
			}
			for _, e := range entry.Value.Map.Entries {
				if e.Key == nil || e.Key.Reference == nil {
					return errors.New("invalid attribute of `custom` (method `" + m.Name + "`)")
				}
				if e.Value == nil || e.Value.String == nil {
					return errors.New("invalid value provided of `" + *e.Key.Reference + "` (method `" + m.Name + "`)")
				}
				if *e.Key.Reference == "kind" {
					b.Method = *e.Value.String
				} else if *e.Key.Reference == "path" {
					b.PathRaw = *e.Value.String
				}
			}
			if b.Method == "" || b.PathRaw == "" {
				return errors.New("http binding incomplete (method `" + m.Name + "`)")
			}
		case "additional_bindings":
			if entry.Value == nil || entry.Value.Map == nil {
				return errors.New("invalid value provided of `" + *entry.Key.String + "` (method `" + m.Name + "`)")
			}
			err := m.parseBinding(entry.Value.Map.Entries)
			if err != nil {
				return err
			}
		}
	}
	var err error
	b.Path, err = io.ParsePath(b.PathRaw)
	if err != nil {
		return errors2.Wrap(err, "failed to parse path `"+b.PathRaw+"`")
	}
	m.HttpBindings = append(m.HttpBindings, b)

	return nil
}

func (d Definition) serviceFromProto(service *io.Service) (*Service, error) {
	s := &Service{
		Service: service,
		Name:    service.Name,
		Methods: make([]*Method, 0),
	}
	for _, entry := range service.Entries {
		if entry.Method != nil {
			m, err := d.methodFromProto(s, entry.Method)
			if err != nil {
				return nil, err
			}
			s.Methods = append(s.Methods, m)
		} else if entry.Option != nil {
			if entry.Option.Name == "config" {
				if entry.Option.Value == nil || entry.Option.Value.Map == nil {
					return nil, errors.New("invalid value provided for `(httpConfig)`")
				}
				for _, mapEntry := range entry.Option.Value.Map.Entries {
					switch *mapEntry.Key.Reference {
					case "HttpPrefix":
						s.HttpPrefix = *mapEntry.Value.String
					case "HttpCompress":
						if mapEntry.Value != nil && mapEntry.Value.Bool != nil {
							s.Compressed = ref(bool(*mapEntry.Value.Bool))
						}
					case "WebSocketPath":
						s.WSPath = *mapEntry.Value.String
					case "WebSocketByDefault":
						if mapEntry.Value != nil && mapEntry.Value.Bool != nil {
							s.WSDefault = ref(bool(*mapEntry.Value.Bool))
						}
					}
				}
			}
		}
	}
	return s, nil
}

func DefinitionFromProto(data *io.Proto) (*Definition, error) {
	d := &Definition{
		services:    make([]*io.Service, 0),
		imports:     make([]string, 0),
		enums:       make([]*io.Enum, 0),
		enumsMap:    make(map[string]*io.Enum),
		messages:    make([]*io.Message, 0),
		messagesMap: make(map[string]*io.Message),
	}

	for _, entry := range data.Entries {
		if entry.Message != nil {
			d.messages = append(d.messages, entry.Message)
			d.messagesMap[entry.Message.Name] = entry.Message
		} else if entry.Enum != nil {
			d.enums = append(d.enums, entry.Enum)
		} else if entry.Service != nil {
			d.services = append(d.services, entry.Service)
		} else if entry.Syntax != "" {
			d.syntax = entry.Syntax
		} else if entry.Package != "" {
			d.pack = entry.Package
		} else if entry.Import != "" {
			d.imports = append(d.imports, entry.Import)
		}
	}

	d.Services = make([]*Service, len(d.services))
	for i, service := range d.services {
		s, err := d.serviceFromProto(service)
		if err != nil {
			return nil, err
		}
		err = s.CheckParams(d)
		if err != nil {
			return nil, err
		}
		d.Services[i] = s
	}

	return d, nil
}

func ref[T any](v T) *T {
	return &v
}
