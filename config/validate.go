package config

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/coldbrewcloud/coldbrew-cli/core"
	"github.com/coldbrewcloud/coldbrew-cli/utils"
	"github.com/coldbrewcloud/coldbrew-cli/utils/conv"
)

func (c *Config) Validate() error {
	if !core.AppNameRE.MatchString(conv.S(c.Name)) {
		return fmt.Errorf("Invalid app name [%s]", conv.S(c.Name))
	}

	if !core.ClusterNameRE.MatchString(conv.S(c.ClusterName)) {
		return fmt.Errorf("Invalid cluster name [%s]", conv.S(c.ClusterName))
	}

	if conv.U16(c.Units) > core.MaxAppUnits {
		return fmt.Errorf("Units cannot exceed %d", core.MaxAppUnits)
	}

	if conv.F64(c.CPU) == 0 {
		return errors.New("CPU cannot be 0")
	}
	if conv.F64(c.CPU) > core.MaxAppCPU {
		return fmt.Errorf("CPU cannot exceed %d", core.MaxAppCPU)
	}

	if !core.SizeExpressionRE.MatchString(conv.S(c.Memory)) {
		return fmt.Errorf("Invalid app memory [%s] (1)", conv.S(c.Memory))
	} else {
		m := core.SizeExpressionRE.FindAllStringSubmatch(conv.S(c.Memory), -1)
		if len(m) != 1 || len(m[0]) < 2 {
			return fmt.Errorf("Invalid app memory [%s] (2)", conv.S(c.Memory))
		}
		parsed, err := strconv.ParseUint(m[0][1], 10, 64)
		if err != nil {
			return fmt.Errorf("Invalid app memory [%s] (3)", conv.S(c.Memory))
		}
		if parsed == 0 {
			return errors.New("App memory cannot be 0")
		}
		if parsed > core.MaxAppMemory {
			return fmt.Errorf("App memory cannot exceed %d", core.MaxAppMemory)
		}
	}

	if !core.TimeExpressionRE.MatchString(conv.S(c.LoadBalancer.HealthCheck.Interval)) {
		return fmt.Errorf("Invalid health check interval [%s]", conv.S(c.LoadBalancer.HealthCheck.Interval))
	}

	if !core.HealthCheckPathRE.MatchString(conv.S(c.LoadBalancer.HealthCheck.Path)) {
		return fmt.Errorf("Invalid health check path [%s]", conv.S(c.LoadBalancer.HealthCheck.Path))
	}

	if !core.HealthCheckStatusRE.MatchString(conv.S(c.LoadBalancer.HealthCheck.Status)) {
		return fmt.Errorf("Invalid health check status [%s]", conv.S(c.LoadBalancer.HealthCheck.Status))
	}

	if !core.TimeExpressionRE.MatchString(conv.S(c.LoadBalancer.HealthCheck.Timeout)) {
		return fmt.Errorf("Invalid health check timeout [%s]", conv.S(c.LoadBalancer.HealthCheck.Timeout))
	}

	if conv.U16(c.LoadBalancer.HealthCheck.HealthyLimit) == 0 {
		return errors.New("Health check healthy limit cannot be 0.")
	}

	if conv.U16(c.LoadBalancer.HealthCheck.UnhealthyLimit) == 0 {
		return errors.New("Health check unhealthy limit cannot be 0.")
	}

	if !core.ECRRepoNameRE.MatchString(conv.S(c.AWS.ECRRepositoryName)) {
		return fmt.Errorf("Invalid ECR Resitory name [%s]", conv.S(c.AWS.ECRRepositoryName))
	}

	if !core.ELBNameRE.MatchString(conv.S(c.AWS.ELBLoadBalancerName)) {
		return fmt.Errorf("Invalid ELB load balancer name [%s]", conv.S(c.AWS.ELBLoadBalancerName))
	}

	if !core.ELBTargetNameRE.MatchString(conv.S(c.AWS.ELBTargetGroupName)) {
		return fmt.Errorf("Invalid ELB target group name [%s]", conv.S(c.AWS.ELBTargetGroupName))
	}

	if utils.IsBlank(conv.S(c.Docker.Bin)) {
		return fmt.Errorf("Invalid docker executable path [%s]", conv.S(c.Docker.Bin))
	}

	return nil
}
