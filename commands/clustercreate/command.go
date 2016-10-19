package clustercreate

import (
	"errors"
	"fmt"
	"strings"

	"time"

	"github.com/coldbrewcloud/coldbrew-cli/aws"
	"github.com/coldbrewcloud/coldbrew-cli/aws/ec2"
	"github.com/coldbrewcloud/coldbrew-cli/config"
	"github.com/coldbrewcloud/coldbrew-cli/console"
	"github.com/coldbrewcloud/coldbrew-cli/core/clusters"
	"github.com/coldbrewcloud/coldbrew-cli/flags"
	"github.com/coldbrewcloud/coldbrew-cli/utils"
	"github.com/coldbrewcloud/coldbrew-cli/utils/conv"
	"github.com/d5/cc"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	defaultInstanceType = "t2.micro"
)

type Command struct {
	globalFlags    *flags.GlobalFlags
	commandFlags   *Flags
	awsClient      *aws.Client
	clusterNameArg *string
}

func (c *Command) Init(ka *kingpin.Application, globalFlags *flags.GlobalFlags) *kingpin.CmdClause {
	c.globalFlags = globalFlags

	cmd := ka.Command("cluster-create", "(cluster-create description goes here)")
	c.commandFlags = NewFlags(cmd)

	c.clusterNameArg = cmd.Arg("cluster-name", "Cluster name").Required().String()

	return cmd
}

func (c *Command) Run(cfg *config.Config) error {
	c.awsClient = aws.NewClient(conv.S(c.globalFlags.AWSRegion), conv.S(c.globalFlags.AWSAccessKey), conv.S(c.globalFlags.AWSSecretKey))

	// AWS networking
	_, vpcID, subnetIDs, err := c.getAWSNetwork()
	if err != nil {
		return c.exitWithError(err)
	}

	clusterName := strings.TrimSpace(conv.S(c.clusterNameArg))

	console.Println("Identifying resources to create...")
	createECSCluster := false
	createECSServiceRole := false
	createInstanceProfile := false
	createInstanceSecurityGroup := false
	createLaunchConfiguration := false
	createAutoScalingGroup := false

	// ECS cluster
	ecsClusterName := clusters.DefaultECSClusterName(clusterName)
	ecsCluster, err := c.awsClient.ECS().RetrieveCluster(ecsClusterName)
	if err != nil {
		return c.exitWithError(fmt.Errorf("Failed to retrieve ECS Cluster [%s]: %s", ecsClusterName, err.Error()))
	}
	if ecsCluster == nil || conv.S(ecsCluster.Status) == "INACTIVE" {
		createECSCluster = true
		console.Println(" ", cc.BlackH("ECS Cluster"), cc.Green(ecsClusterName))
	}

	// ECS service role
	ecsServiceRoleName := clusters.DefaultECSServiceRoleName(clusterName)
	ecsServiceRole, err := c.awsClient.IAM().RetrieveRole(ecsServiceRoleName)
	if err != nil {
		return c.exitWithError(fmt.Errorf("Failed to retrieve IAM Role [%s]: %s", ecsServiceRoleName, err.Error()))
	}
	if ecsServiceRole == nil {
		createECSServiceRole = true
		console.Println(" ", cc.BlackH("ECS Service Rike"), cc.Green(ecsServiceRoleName))
	}

	// launch configuration
	lcName := clusters.DefaultLaunchConfigurationName(clusterName)
	launchConfiguration, err := c.awsClient.AutoScaling().RetrieveLaunchConfiguration(lcName)
	if err != nil {
		return c.exitWithError(fmt.Errorf("Failed to delete Launch Configuration [%s]: %s", lcName, err.Error()))
	}
	if launchConfiguration == nil {
		createLaunchConfiguration = true
		console.Println(" ", cc.BlackH("Launch Config"), cc.Green(lcName))
	}

	// auto scaling group
	asgName := clusters.DefaultAutoScalingGroupName(clusterName)
	autoScalingGroup, err := c.awsClient.AutoScaling().RetrieveAutoScalingGroup(asgName)
	if err != nil {
		return c.exitWithError(fmt.Errorf("Failed to retrieve Auto Scaling Group [%s]: %s", asgName, err.Error()))
	}
	if autoScalingGroup == nil || !utils.IsBlank(conv.S(autoScalingGroup.Status)) {
		createAutoScalingGroup = true
		console.Println(" ", cc.BlackH("Auto Scaling Group"), cc.Green(asgName))
	}

	// instance profile
	instanceProfileName := clusters.DefaultInstanceProfileName(clusterName)
	if !utils.IsBlank(conv.S(c.commandFlags.InstanceProfile)) {
		instanceProfileName = conv.S(c.commandFlags.InstanceProfile)
	} else {
		instanceProfile, err := c.awsClient.IAM().RetrieveInstanceProfile(instanceProfileName)
		if err != nil {
			return c.exitWithError(fmt.Errorf("Failed to retrieve Instance Profile [%s]: %s", instanceProfileName, err.Error()))
		}
		if instanceProfile == nil {
			createInstanceProfile = true
			console.Println(" ", cc.BlackH("Instance Profile"), cc.Green(instanceProfileName))
		}
	}

	// instance security group
	sgName := clusters.DefaultInstnaceSecurityGroupName(clusterName)
	securityGroup, err := c.awsClient.EC2().RetrieveSecurityGroupByName(sgName)
	instanceSecurityGroupID := ""
	if err != nil {
		return c.exitWithError(fmt.Errorf("Failed to retrieve Security Group [%s]: %s", sgName, err.Error()))
	}
	if securityGroup == nil {
		createInstanceSecurityGroup = true
		console.Println(" ", cc.BlackH("Instance Security Group"), cc.Green(sgName))
	} else {
		instanceSecurityGroupID = conv.S(securityGroup.GroupId)
	}

	if !createECSServiceRole && !createECSCluster && !createLaunchConfiguration && !createAutoScalingGroup &&
		!createInstanceProfile && !createInstanceSecurityGroup {
		console.Println("Looks like everything is already up and running!")
		return nil
	}

	// confirmation
	if !conv.B(c.commandFlags.ForceCreate) && !console.AskConfirm("Do you want to create these resources?") {
		return nil
	}

	// create instance profile
	if createInstanceProfile {
		console.Printf("Creating Instance Profile [%s]...\n", cc.Green(instanceProfileName))

		if _, err = c.createDefaultInstanceProfile(instanceProfileName); err != nil {
			return c.exitWithError(fmt.Errorf("Failed to create Instance Profile [%s]: %s", instanceProfileName, err.Error()))
		}
	}

	// create instance security group
	if createInstanceSecurityGroup {
		console.Printf("Creating Security Group [%s]...\n", cc.Green(sgName))

		var err error
		instanceSecurityGroupID, err = c.awsClient.EC2().CreateSecurityGroup(sgName, sgName, vpcID)
		if err != nil {
			return c.exitWithError(fmt.Errorf("Failed to create EC2 Security Group [%s] for container instances: %s", sgName, err.Error()))
		}
		if err := c.awsClient.EC2().AddInboundToSecurityGroup(instanceSecurityGroupID, ec2.SecurityGroupProtocolTCP, 22, 22, "0.0.0.0/0"); err != nil {
			return c.exitWithError(fmt.Errorf("Failed to add SSH inbound rule to Security Group [%s]: %s", sgName, err.Error()))
		}
	}

	// create launch configuration
	if createLaunchConfiguration {
		console.Printf("Creating Launch Configuration [%s]...\n", cc.Green(lcName))

		// key pair
		keyPairName := strings.TrimSpace(conv.S(c.commandFlags.KeyPairName))
		keyPairInfo, err := c.awsClient.EC2().RetrieveKeyPair(keyPairName)
		if err != nil {
			return c.exitWithError(fmt.Errorf("Failed to retrieve key pair info [%s]: %s", keyPairName, err.Error()))
		}
		if keyPairInfo == nil {
			return c.exitWithError(fmt.Errorf("Key pair [%s] was not found\n", keyPairName))
		}

		// container instance type
		instanceType := strings.TrimSpace(conv.S(c.commandFlags.InstanceType))
		if instanceType == "" {
			instanceType = console.AskQuestion("Enter instance type", defaultInstanceType)
		}

		// container instance image ID
		imageID := c.getClusterImageID(conv.S(c.globalFlags.AWSRegion))
		if imageID == "" {
			return c.exitWithError(errors.New("No defatul instance image found"))
		}

		instanceUserData := c.getInstanceUserData(ecsClusterName)

		// NOTE: sometimes resources created (e.g. InstanceProfile) do not become available immediately.
		// So here we retry up to 10 times just to be safe.
		var lastErr error
		for i := 0; i < 10; i++ {
			err := c.awsClient.AutoScaling().CreateLaunchConfiguration(lcName, instanceType, imageID, []string{instanceSecurityGroupID}, keyPairName, instanceProfileName, instanceUserData)
			if err != nil {
				lastErr = err
			} else {
				lastErr = nil
				break
			}

			time.Sleep(1 * time.Second)
		}
		if lastErr != nil {
			return c.exitWithError(fmt.Errorf("Failed to create EC2 Launch Configuration [%s]: %s", lcName, lastErr.Error()))
		}
	}

	// create auto scaling group
	if createAutoScalingGroup {
		console.Printf("Creating Auto Scaling Group [%s]...\n", cc.Green(asgName))

		// if existing auto scaling group is currently pending delete, wait a bit so it gets fully deleted
		if autoScalingGroup != nil && !utils.IsBlank(conv.S(autoScalingGroup.Status)) {
			if err := c.waitAutoScalingGroupDeletion(asgName); err != nil {
				return c.exitWithError(err)
			}
		}

		initialCapacity := conv.U16(c.commandFlags.InitialCapacity)

		err = c.awsClient.AutoScaling().CreateAutoScalingGroup(asgName, lcName, subnetIDs, 0, initialCapacity, initialCapacity)
		if err != nil {
			return c.exitWithError(fmt.Errorf("Failed to create EC2 Auto Scaling Group [%s]: %s", asgName, err.Error()))
		}
	}

	// create ECS cluster
	if createECSCluster {
		console.Printf("Creating ECS Cluster [%s]...\n", cc.Green(ecsClusterName))

		if _, err := c.awsClient.ECS().CreateCluster(ecsClusterName); err != nil {
			return c.exitWithError(fmt.Errorf("Failed to create ECS Cluster [%s]: %s", ecsClusterName, err.Error()))
		}
	}

	// create ECS service role
	if createECSServiceRole {
		console.Printf("Creating ECS Service Role [%s]...\n", cc.Green(ecsServiceRoleName))

		if _, err := c.createECSServiceRole(ecsServiceRoleName); err != nil {
			return c.exitWithError(fmt.Errorf("Failed to create IAM role [%s]: %s", ecsServiceRoleName, err.Error()))
		}
	}

	return nil
}

func (c *Command) exitWithError(err error) error {
	console.Errorln(cc.Red("Error:"), err.Error())
	return nil
}
