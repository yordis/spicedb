package v1

import (
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"

	"github.com/authzed/spicedb/pkg/caveats"
	"github.com/authzed/spicedb/pkg/spiceerrors"
	"github.com/authzed/spicedb/pkg/tuple"
)

func computeCheckBulkPermissionsItemHashWithoutResourceID(req *v1.CheckBulkPermissionsRequestItem) (string, error) {
	return computeCallHash("v1.checkbulkpermissionsrequestitem", nil, map[string]any{
		"resource-type":    req.Resource.ObjectType,
		"permission":       req.Permission,
		"subject-type":     req.Subject.Object.ObjectType,
		"subject-id":       req.Subject.Object.ObjectId,
		"subject-relation": req.Subject.OptionalRelation,
		"context":          req.Context,
	})
}

func computeCheckBulkPermissionsItemHash(req *v1.CheckBulkPermissionsRequestItem) (string, error) {
	return computeCallHash("v1.checkbulkpermissionsrequestitem", nil, map[string]any{
		"resource-type":    req.Resource.ObjectType,
		"resource-id":      req.Resource.ObjectId,
		"permission":       req.Permission,
		"subject-type":     req.Subject.Object.ObjectType,
		"subject-id":       req.Subject.Object.ObjectId,
		"subject-relation": req.Subject.OptionalRelation,
		"context":          req.Context,
	})
}

func computeReadRelationshipsRequestHash(req *v1.ReadRelationshipsRequest) (string, error) {
	osf := req.RelationshipFilter.OptionalSubjectFilter
	if osf == nil {
		osf = &v1.SubjectFilter{}
	}

	srf := "(none)"
	if osf.OptionalRelation != nil {
		srf = osf.OptionalRelation.Relation
	}

	return computeCallHash("v1.readrelationships", req.Consistency, map[string]any{
		"filter-resource-type": req.RelationshipFilter.ResourceType,
		"filter-relation":      req.RelationshipFilter.OptionalRelation,
		"filter-resource-id":   req.RelationshipFilter.OptionalResourceId,
		"subject-type":         osf.SubjectType,
		"subject-relation":     srf,
		"subject-resource-id":  osf.OptionalSubjectId,
		"limit":                req.OptionalLimit,
	})
}

func computeLRRequestHash(req *v1.LookupResourcesRequest) (string, error) {
	return computeCallHash("v1.lookupresources", req.Consistency, map[string]any{
		"resource-type": req.ResourceObjectType,
		"permission":    req.Permission,
		"subject":       tuple.V1StringSubjectRef(req.Subject),
		"limit":         req.OptionalLimit,
		"context":       req.Context,
	})
}

func computeWriteRelationshipsRequestHash(req *v1.WriteRelationshipsRequest) (string, error) {
	updateStrings := make([]string, len(req.Updates))
	for i, update := range req.Updates {
		updateString, err := writeUpdateStringForHash(update)
		if err != nil {
			return "", err
		}
		updateStrings[i] = updateString
	}
	sort.Strings(updateStrings)

	preconditionStrings := make([]string, len(req.OptionalPreconditions))
	for i, precond := range req.OptionalPreconditions {
		preconditionStrings[i] = precond.String()
	}
	sort.Strings(preconditionStrings)

	return computeCallHash("v1.writerelationships", nil, map[string]any{
		"updates":       strings.Join(updateStrings, ","),
		"preconditions": strings.Join(preconditionStrings, ","),
		"metadata":      req.OptionalTransactionMetadata,
	})
}

func writeUpdateStringForHash(update *v1.RelationshipUpdate) (string, error) {
	if update == nil {
		return "", nil
	}

	relString, err := relationshipStringForHash(update.Relationship)
	if err != nil {
		return "", err
	}

	opName := v1.RelationshipUpdate_Operation_name[int32(update.Operation)]
	return opName + ":" + relString, nil
}

func relationshipStringForHash(rel *v1.Relationship) (string, error) {
	if rel == nil || rel.Resource == nil || rel.Subject == nil {
		return "", nil
	}

	relationship := tuple.V1StringRelationshipWithoutCaveatOrExpiration(rel)
	if relationship == "" {
		return "", nil
	}

	if rel.OptionalCaveat != nil && rel.OptionalCaveat.CaveatName != "" {
		contextString, err := caveats.StableContextStringForHashing(rel.OptionalCaveat.Context)
		if err != nil {
			return "", err
		}
		if len(contextString) > 0 {
			contextString = ":" + contextString
		}
		relationship += "[" + rel.OptionalCaveat.CaveatName + contextString + "]"
	}

	if rel.OptionalExpiresAt != nil {
		expirationString, err := tuple.V1StringExpiration(rel.OptionalExpiresAt)
		if err != nil {
			return "", err
		}
		relationship += expirationString
	}

	return relationship, nil
}

func computeCallHash(apiName string, consistency *v1.Consistency, arguments map[string]any) (string, error) {
	stringArguments := make(map[string]string, len(arguments)+1)

	if consistency == nil {
		consistency = &v1.Consistency{
			Requirement: &v1.Consistency_MinimizeLatency{
				MinimizeLatency: true,
			},
		}
	}

	consistencyBytes, err := consistency.MarshalVT()
	if err != nil {
		return "", err
	}

	stringArguments["consistency"] = string(consistencyBytes)

	for argName, argValue := range arguments {
		if argName == "consistency" {
			return "", spiceerrors.MustBugf("cannot specify consistency in the arguments")
		}

		switch v := argValue.(type) {
		case string:
			stringArguments[argName] = v

		case int:
			stringArguments[argName] = strconv.Itoa(v)

		case uint32:
			stringArguments[argName] = strconv.Itoa(int(v))

		case *structpb.Struct:
			stringArguments[argName] = caveats.StableContextStringForHashing(v)

		default:
			return "", spiceerrors.MustBugf("unknown argument type in compute call hash")
		}
	}
	return computeAPICallHash(apiName, stringArguments)
}
