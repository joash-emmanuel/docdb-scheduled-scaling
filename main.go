package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	"github.com/aws/aws-sdk-go-v2/service/docdb/types"
)

//filters = https://stackoverflow.com/questions/54026903/filter-aws-resources-using-regex-in-aws-sdk-go

func main() {
	cfg := createsession()
	// add_instance(cfg)
	describe_db_instances(cfg)
}

var Instance_name_prefix = "temp-instance"
var Cluster_identifier = "eu-west-1-test-cluster-1"
var Instance_class = "db.t3.medium"
var Current_number_of_instances int
var Current_cluster_member_names = []string{}
var Number_of_instances_to_add = 2
var Metadata_for_deletion []byte

func createsession() aws.Config {

	// Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-west-2"),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	return cfg
}

func describe_db_cluster(cfg aws.Config) (Current_number_of_instances int, Current_cluster_member_names []string) {

	// Using the Config value, create the docdb client
	docdb_client := docdb.NewFromConfig(cfg)

	describe_cluster_output, err := docdb_client.DescribeDBClusters(context.TODO(), &docdb.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(Cluster_identifier), //If this parameter is specified, information. // from only the specific cluster is returned
		Filters: []types.Filter{
			{
				Name:   aws.String("db-cluster-id"),
				Values: []string{Cluster_identifier},
			},
		},
	})

	if err != nil {
		panic(err)
	}

	for _, cluster_description_output := range describe_cluster_output.DBClusters {
		//find the current number of instances available in the cluster
		Current_number_of_instances = len(cluster_description_output.DBClusterMembers)

		// fmt.Printf("%+v\n", object.DBClusterMembers)

		//Find the instance id/names in the cluster
		for _, cluster_member_ids := range cluster_description_output.DBClusterMembers {
			// fmt.Println(aws.ToString(cluster_member_ids.DBInstanceIdentifier)) //lists the instance names before the creation
			Current_cluster_member_names = append(Current_cluster_member_names, aws.ToString(cluster_member_ids.DBInstanceIdentifier))
		}
	}
	// fmt.Println(Current_cluster_member_names)

	return

}

func add_instance(cfg aws.Config) {

	for i := 0; i < Number_of_instances_to_add; i++ {

		//Import the int current number of instances from the describe_db_cluster function
		Current_number_of_instances, _ := describe_db_cluster(cfg)

		fmt.Printf("The cluster currently has a total of %v instances before this creation\n", Current_number_of_instances)

		if Current_number_of_instances+1 > 15 {
			log.Panic("The number of instances cannot be more than 15")
		} else {

			//define the db instance identifier based on the instance-name-prefix and currentlocaltime
			current_time := time.Now().Local().Format("20060102150405") //yyyymmddhhmmss
			Instance_identifier := Instance_name_prefix + "-" + current_time

			//create a new docdb client based on the session created
			docdb_client := docdb.NewFromConfig(cfg)

			create_db_output, err := docdb_client.CreateDBInstance(context.TODO(), &docdb.CreateDBInstanceInput{
				DBClusterIdentifier:  aws.String(Cluster_identifier),  // The identifier of the cluster that the instance will belong to.
				DBInstanceClass:      aws.String(Instance_class),      // DBInstanceClass is a required field eg r5.2xlarge
				DBInstanceIdentifier: aws.String(Instance_identifier), // The instance identifier. This parameter is stored as a lowercase string
				Engine:               aws.String("docdb"),             // The name of the database engine to be used for this instance.
				Tags: []types.Tag{
					{
						Key:   aws.String("ScheduledTempInstance"),
						Value: aws.String("Yes"),
					},
				},
			})

			if err != nil {
				panic(err)
			}
			//Retrieve instance name from the output
			Instance_name := aws.ToString(create_db_output.DBInstance.DBInstanceIdentifier)
			log.Printf("Instance %v has been created", Instance_name)

		}
	}
}

func describe_db_instances(cfg aws.Config) (Metadata_json_conversion []byte) {
	//create a map to feed the instance name for the temp-instance and the time it was created
	instance_metadata := make(map[string]interface{})

	//Import the Slice name of current instances from the describe_db_cluster function
	_, Current_cluster_member_names := describe_db_cluster(cfg)

	//create a new docdb client based on the session created
	docdb_client := docdb.NewFromConfig(cfg)

	//Iterate through the cluster instance members, filter the members with the names they start with and get the time of creation for the instances
	for _, temporary_created_nodes := range Current_cluster_member_names {
		if strings.HasPrefix(temporary_created_nodes, "temp-instance") {
			describe_instances_output, err := docdb_client.DescribeDBInstances(context.TODO(), &docdb.DescribeDBInstancesInput{
				//* If provided, must match the identifier of an existing DBInstance.
				DBInstanceIdentifier: aws.String(temporary_created_nodes),
				Filters: []types.Filter{
					{
						Name:   aws.String("db-cluster-id"),
						Values: []string{Cluster_identifier},
					},
				},
			})

			if err != nil {
				panic(err)
			}

			//Get the time of instance creation
			for _, Instance_information := range describe_instances_output.DBInstances {
				// for _, status_of_instance := range time_of_creation.StatusInfos {

				// if status_of_instance.StatusType == aws.String("read replication") {
				key := temporary_created_nodes
				value := Instance_information.InstanceCreateTime
				instance_metadata[key] = value
				// instance_metadata[temporary_created_nodes]=Instance_information.InstanceCreateTime
				// }
			}

		}

	}

	//convert the map to json
	Metadata_for_deletion, err := json.Marshal(instance_metadata)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(Metadata_for_deletion))

	return

}

var Number_of_instances_to_delete = 2

func delete_instance(cfg aws.Config, metadata_json_conversion []byte) {

	metadata_json_conversion = describe_db_instances(cfg)

	var deleteion_criteria_data map[string]interface{}
	err := json.Unmarshal(Metadata_for_deletion, &deleteion_criteria_data)
	if err != nil {
		panic(err)
	}

	// Loop through the keys
	for instance_name, creation_time := range deleteion_criteria_data {
		fmt.Printf("%s: %v\n", instance_name, creation_time)

	}

	for i := 0; i < Number_of_instances_to_delete; i++ {

		//create a new docdb client based on the session created
		docdb_client := docdb.NewFromConfig(cfg)

		//    * Must match the name of an existing instance.

		delete_instance_output, err := docdb_client.DeleteDBInstance(context.TODO(), &docdb.DeleteDBInstanceInput{
			DBInstanceIdentifier: aws.String(""),
		})

		if err != nil {
			panic(err)
		}

	}

}
