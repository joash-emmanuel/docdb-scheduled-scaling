package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	"github.com/aws/aws-sdk-go-v2/service/docdb/types"
)

//sdk -- https://docs.aws.amazon.com/sdk-for-go/api/service/docdb/

func main() {
	cfg := createsession()
	// add_instance(cfg)
	// describe_db_instances(cfg)
	delete_instance(cfg)
}

var Cluster_identifier = "eu-west-1-test-cluster-1"
var Instance_class = "db.t3.medium"
var Number_of_instances_to_add = 3
var Number_of_instances_to_delete = 2
var Instance_name_prefix = "temp-instance"
var Current_number_of_instances int
var Current_cluster_member_names = []string{}
var Temp_Instances = []string{}

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

func describe_db_instances(cfg aws.Config) (Temp_Instances []string) {

	//Import the Slice name of current instances from the describe_db_cluster function
	_, Current_cluster_member_names := describe_db_cluster(cfg)

	//create a new docdb client based on the session created
	docdb_client := docdb.NewFromConfig(cfg)

	//Iterate through the cluster instance members, filter the members with the names temp-instance. Some type of iteration has been done here
	for _, temporary_created_nodes_ids := range Current_cluster_member_names {
		if strings.HasPrefix(temporary_created_nodes_ids, "temp-instance") {
			describe_instances_output, err := docdb_client.DescribeDBInstances(context.TODO(), &docdb.DescribeDBInstancesInput{
				//* If provided, must match the identifier of an existing DBInstance.
				DBInstanceIdentifier: aws.String(temporary_created_nodes_ids),
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

			for _, instance_names := range describe_instances_output.DBInstances {
				availability_status := aws.ToString(instance_names.DBInstanceStatus) //Get the availability status of the instances
				// fmt.Println(availability_status) 	//status of instances = creating, available, deleting
				if availability_status == "available" {
					Temp_Instances = append(Temp_Instances, aws.ToString(instance_names.DBInstanceIdentifier))
				}

			}

		}

	}

	// Only print once at the end if nothing was added
	if len(Temp_Instances) == 0 {
		log.Panic("No instances to be deleted")
	}

	return
}

func delete_instance(cfg aws.Config) {

	//create a new docdb client based on the session created
	docdb_client := docdb.NewFromConfig(cfg)

	for i := 0; i < Number_of_instances_to_delete; i++ {

		Temp_Instances := describe_db_instances(cfg)

		//for _, instance_to_be_deleted := range Temp_Instances { //deletes all the instances as i have passed all the instance ids to instance to be deleted

		instance_to_be_deleted := Temp_Instances[i] //deletes one by one based on how many times to for loop runs

		//    * Must match the name of an existing instance.
		delete_instance_output, err := docdb_client.DeleteDBInstance(context.TODO(), &docdb.DeleteDBInstanceInput{
			DBInstanceIdentifier: aws.String(instance_to_be_deleted),
		})

		if err != nil {
			panic(err)
		}

		deleted_instance_name := aws.ToString(delete_instance_output.DBInstance.DBInstanceIdentifier)
		fmt.Printf("Instance %v is being deleted\n", deleted_instance_name)
		//}
	}
}
