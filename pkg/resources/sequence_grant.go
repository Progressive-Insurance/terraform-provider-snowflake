package resources

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/pkg/errors"

	"github.com/chanzuckerberg/terraform-provider-snowflake/pkg/snowflake"
)

var validSequencePrivileges = newPrivilegeSet(
	privilegeOwnership,
	privilegeUsage,
)

var sequenceGrantSchema = map[string]*schema.Schema{
	"sequence_name": {
		Type:        schema.TypeString,
		Optional:    true,
		Description: "The name of the sequence on which to grant privileges immediately (only valid if on_future is unset).",
		ForceNew:    true,
	},
	"schema_name": {
		Type:        schema.TypeString,
		Optional:    true,
		Description: "The name of the schema containing the current or future sequences on which to grant privileges.",
		ForceNew:    true,
	},
	"database_name": {
		Type:        schema.TypeString,
		Required:    true,
		Description: "The name of the database containing the current or future sequences on which to grant privileges.",
		ForceNew:    true,
	},
	"privilege": {
		Type:         schema.TypeString,
		Optional:     true,
		Description:  "The privilege to grant on the current or future sequence.",
		Default:      "USAGE",
		ValidateFunc: validation.StringInSlice(validSequencePrivileges.toList(), true),
		ForceNew:     true,
	},
	"roles": {
		Type:        schema.TypeSet,
		Elem:        &schema.Schema{Type: schema.TypeString},
		Optional:    true,
		Description: "Grants privilege to these roles.",
		ForceNew:    true,
	},
	"on_future": {
		Type:          schema.TypeBool,
		Optional:      true,
		Description:   "When this is set to true and a schema_name is provided, apply this grant on all future sequences in the given schema. When this is true and no schema_name is provided apply this grant on all future sequences in the given database. The sequence_name field must be unset in order to use on_future.",
		Default:       false,
		ForceNew:      true,
		ConflictsWith: []string{"sequence_name"},
	},
	"with_grant_option": {
		Type:        schema.TypeBool,
		Optional:    true,
		Description: "When this is set to true, allows the recipient role to grant the privileges to other roles.",
		Default:     false,
		ForceNew:    true,
	},
}

// SequenceGrant returns a pointer to the resource representing a sequence grant
func SequenceGrant() *schema.Resource {
	return &schema.Resource{
		Create: CreateSequenceGrant,
		Read:   ReadSequenceGrant,
		Delete: DeleteSequenceGrant,

		Schema: sequenceGrantSchema,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
	}
}

// CreateSequenceGrant implements schema.CreateFunc
func CreateSequenceGrant(data *schema.ResourceData, meta interface{}) error {
	var (
		sequenceName string
		schemaName   string
	)
	if _, ok := data.GetOk("sequence_name"); ok {
		sequenceName = data.Get("sequence_name").(string)
	}
	if _, ok := data.GetOk("schema_name"); ok {
		schemaName = data.Get("schema_name").(string)
	}
	dbName := data.Get("database_name").(string)
	priv := data.Get("privilege").(string)
	futureSequences := data.Get("on_future").(bool)
	grantOption := data.Get("with_grant_option").(bool)

	if (schemaName == "") && !futureSequences {
		return errors.New("schema_name must be set unless on_future is true.")
	}

	if (sequenceName == "") && !futureSequences {
		return errors.New("sequence_name must be set unless on_future is true.")
	}
	if (sequenceName != "") && futureSequences {
		return errors.New("sequence_name must be empty if on_future is true.")
	}

	var builder snowflake.GrantBuilder
	if futureSequences {
		builder = snowflake.FutureSequenceGrant(dbName, schemaName)
	} else {
		builder = snowflake.SequenceGrant(dbName, schemaName, sequenceName)
	}

	err := createGenericGrant(data, meta, builder)
	if err != nil {
		return err
	}

	grant := &grantID{
		ResourceName: dbName,
		SchemaName:   schemaName,
		ObjectName:   sequenceName,
		Privilege:    priv,
		GrantOption:  grantOption,
	}
	dataIDInput, err := grant.String()
	if err != nil {
		return err
	}
	data.SetId(dataIDInput)

	return ReadSequenceGrant(data, meta)
}

// ReadSequenceGrant implements schema.ReadFunc
func ReadSequenceGrant(data *schema.ResourceData, meta interface{}) error {
	grantID, err := grantIDFromString(data.Id())
	if err != nil {
		return err
	}
	dbName := grantID.ResourceName
	schemaName := grantID.SchemaName
	sequenceName := grantID.ObjectName
	priv := grantID.Privilege

	err = data.Set("database_name", dbName)
	if err != nil {
		return err
	}
	err = data.Set("schema_name", schemaName)
	if err != nil {
		return err
	}
	futureSequencesEnabled := false
	if sequenceName == "" {
		futureSequencesEnabled = true
	}
	err = data.Set("sequence_name", sequenceName)
	if err != nil {
		return err
	}
	err = data.Set("on_future", futureSequencesEnabled)
	if err != nil {
		return err
	}
	err = data.Set("privilege", priv)
	if err != nil {
		return err
	}
	err = data.Set("with_grant_option", grantID.GrantOption)
	if err != nil {
		return err
	}

	var builder snowflake.GrantBuilder
	if futureSequencesEnabled {
		builder = snowflake.FutureSequenceGrant(dbName, schemaName)
	} else {
		builder = snowflake.SequenceGrant(dbName, schemaName, sequenceName)
	}

	return readGenericGrant(data, meta, builder, futureSequencesEnabled, validSequencePrivileges)
}

// DeleteSequenceGrant implements schema.DeleteFunc
func DeleteSequenceGrant(data *schema.ResourceData, meta interface{}) error {
	grantID, err := grantIDFromString(data.Id())
	if err != nil {
		return err
	}
	dbName := grantID.ResourceName
	schemaName := grantID.SchemaName
	sequenceName := grantID.ObjectName

	futureSequences := (sequenceName == "")

	var builder snowflake.GrantBuilder
	if futureSequences {
		builder = snowflake.FutureSequenceGrant(dbName, schemaName)
	} else {
		builder = snowflake.SequenceGrant(dbName, schemaName, sequenceName)
	}
	return deleteGenericGrant(data, meta, builder)
}