package pipelines

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseapi "github.com/konflux-ci/release-service/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	fbcServiceAccountName   = "release-service-account"
	fbcSourceGitURL         = "https://github.com/redhat-appstudio-qe/fbc-sample-repo"
	fbcGitSrcSHA            = "2b04501c777aa4f7ad80f3e233731f3201e5b21b"
	fbcDockerFilePath       = "catalog.Dockerfile"
	targetPort              = 50051
	relSvcCatalogPathInRepo = "pipelines/fbc-release/fbc-release.yaml"
)

var snapshot *appservice.Snapshot
var releaseCR *releaseapi.Release
var err error
var devWorkspace = utils.GetEnv(constants.RELEASE_DEV_WORKSPACE_ENV, constants.DevReleaseTeam)
var managedWorkspace = utils.GetEnv(constants.RELEASE_MANAGED_WORKSPACE_ENV, constants.ManagedReleaseTeam)
var devFw *framework.Framework
var mFw *framework.Framework
var managedFw *framework.Framework

var _ = framework.ReleasePipelinesSuiteDescribe("FBC e2e-tests", Label("release-pipelines", "fbc-tests"), func() {
	defer GinkgoRecover()

	var devNamespace = devWorkspace + "-tenant"
	var managedNamespace = managedWorkspace + "-tenant"

	var issueId = "bz12345"
	var productName = "preGA-product"
	var productVersion = "v2"
	var fbcApplicationName = "fbc-pipelines-app-" + util.GenerateRandomString(4)
	var fbcHotfixAppName = "fbc-hotfix-app-" + util.GenerateRandomString(4)
	var fbcPreGAAppName = "fbc-prega-app-" + util.GenerateRandomString(4)
	var fbcComponentName = "fbc-pipelines-comp-" + util.GenerateRandomString(4)
	var fbcHotfixCompName = "fbc-hotfix-comp-" + util.GenerateRandomString(4)
	var fbcPreGACompName = "fbc-prega-comp-" + util.GenerateRandomString(4)
	var fbcReleasePlanName = "fbc-pipelines-rp-" + util.GenerateRandomString(4)
	var fbcHotfixRPName = "fbc-hotfix-rp-" + util.GenerateRandomString(4)
	var fbcPreGARPName = "fbc-prega-rp-" + util.GenerateRandomString(4)
	var fbcReleasePlanAdmissionName = "fbc-pipelines-rpa-" + util.GenerateRandomString(4)
	var fbcHotfixRPAName = "fbc-hotfix-rpa-" + util.GenerateRandomString(4)
	var fbcPreGARPAName = "fbc-prega-rpa-" + util.GenerateRandomString(4)
	var fbcEnterpriseContractPolicyName = "fbc-pipelines-policy-" + util.GenerateRandomString(4)
	var fbcHotfixECPolicyName = "fbc-hotfix-policy-" + util.GenerateRandomString(4)
	var fbcPreGAECPolicyName = "fbc-prega-policy-" + util.GenerateRandomString(4)
	var sampleImage	= "quay.io/redhat-user-workloads-stage/dev-release-team-tenant/e2e-fbc-app/fbc-sample-repo@sha256:857814679c1deec5bc5d6d8064832b4c0af40dcb07dad57c48f23e5ba6926aed"

	AfterEach(framework.ReportFailure(&devFw))

	Describe("with FBC happy path", Label("fbcHappyPath"), func() {
		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)

			managedNamespace = managedFw.UserNamespace

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcApplicationName, devNamespace)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Println("Created application :", fbcApplicationName)

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(fbcReleasePlanName, devNamespace, fbcApplicationName, managedNamespace, "true", nil, nil)
			Expect(err).NotTo(HaveOccurred())

			createFBCReleasePlanAdmission(fbcReleasePlanAdmissionName, *managedFw, devNamespace, managedNamespace, fbcApplicationName, fbcEnterpriseContractPolicyName, relSvcCatalogPathInRepo, "false", "", "", "", "")

			createFBCEnterpriseContractPolicy(fbcEnterpriseContractPolicyName, *managedFw, devNamespace, managedNamespace)
			snapshot, err = releasecommon.CreateSnapshotWithImageSource(*devFw, fbcComponentName, fbcApplicationName, devNamespace, sampleImage, fbcSourceGitURL, fbcGitSrcSHA, "", "", "", "")
                        Expect(err).ShouldNot(HaveOccurred())

		})

		AfterAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(fbcApplicationName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(fbcEnterpriseContractPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred())
		})

		var _ = Describe("Post-release verification", func() {

			It("verifies the fbc release pipelinerun is running and succeeds", func() {
				assertReleasePipelineRunSucceeded(*devFw, *managedFw, devNamespace, managedNamespace, fbcApplicationName, fbcComponentName)
			})

			It("verifies release CR completed and set succeeded.", func() {
				assertReleaseCRSucceeded(*devFw, devNamespace, managedNamespace, fbcApplicationName, fbcComponentName)
			})
		})
	})

	Describe("with FBC hotfix process", Label("fbcHotfix"), func() {

		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcHotfixAppName, devNamespace)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Println("Created application :", fbcHotfixAppName)

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(fbcHotfixRPName, devNamespace, fbcHotfixAppName, managedNamespace, "true", nil, nil)
			Expect(err).NotTo(HaveOccurred())

			createFBCReleasePlanAdmission(fbcHotfixRPAName, *managedFw, devNamespace, managedNamespace, fbcHotfixAppName, fbcHotfixECPolicyName, relSvcCatalogPathInRepo, "true", issueId, "false", "", "")

			createFBCEnterpriseContractPolicy(fbcHotfixECPolicyName, *managedFw, devNamespace, managedNamespace)

			snapshot, err = releasecommon.CreateSnapshotWithImageSource(*devFw, fbcHotfixCompName, fbcHotfixAppName, devNamespace, sampleImage, fbcSourceGitURL, fbcGitSrcSHA, "", "", "", "")
                        Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(fbcHotfixAppName, devNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(fbcHotfixECPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
			Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcHotfixRPAName, managedNamespace, false)).NotTo(HaveOccurred())
		})

		var _ = Describe("FBC hotfix post-release verification", func() {

			It("verifies the fbc release pipelinerun is running and succeeds", func() {
				assertReleasePipelineRunSucceeded(*devFw, *managedFw, devNamespace, managedNamespace, fbcHotfixAppName, fbcHotfixCompName)
			})

			It("verifies release CR completed and set succeeded.", func() {
				assertReleaseCRSucceeded(*devFw, devNamespace, managedNamespace, fbcHotfixAppName, fbcHotfixCompName)
			})
		})
	})

	Describe("with FBC pre-GA process", Label("fbcPreGA"), func() {

		BeforeAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)

			_, err = devFw.AsKubeDeveloper.HasController.CreateApplication(fbcPreGAAppName, devNamespace)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Println("Created application :", fbcPreGAAppName)

			_, err = devFw.AsKubeDeveloper.ReleaseController.CreateReleasePlan(fbcPreGARPName, devNamespace, fbcPreGAAppName, managedNamespace, "true", nil, nil)
			Expect(err).NotTo(HaveOccurred())

			createFBCEnterpriseContractPolicy(fbcPreGAECPolicyName, *managedFw, devNamespace, managedNamespace)
			createFBCReleasePlanAdmission(fbcPreGARPAName, *managedFw, devNamespace, managedNamespace, fbcPreGAAppName, fbcPreGAECPolicyName, relSvcCatalogPathInRepo, "false", issueId, "true", productName, productVersion)

			snapshot, err = releasecommon.CreateSnapshotWithImageSource(*devFw, fbcPreGACompName, fbcPreGAAppName, devNamespace, sampleImage, fbcSourceGitURL, fbcGitSrcSHA, "", "", "", "")
                        Expect(err).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			devFw = releasecommon.NewFramework(devWorkspace)
			managedFw = releasecommon.NewFramework(managedWorkspace)
			if !CurrentSpecReport().Failed() {
				Expect(devFw.AsKubeDeveloper.HasController.DeleteApplication(fbcPreGAAppName, devNamespace, false)).NotTo(HaveOccurred())
				Expect(managedFw.AsKubeDeveloper.TektonController.DeleteEnterpriseContractPolicy(fbcPreGAECPolicyName, managedNamespace, false)).NotTo(HaveOccurred())
				Expect(managedFw.AsKubeDeveloper.ReleaseController.DeleteReleasePlanAdmission(fbcPreGARPAName, managedNamespace, false)).NotTo(HaveOccurred())
			}
		})

		var _ = Describe("FBC pre-GA post-release verification", func() {

			It("verifies the fbc release pipelinerun is running and succeeds", func() {
				assertReleasePipelineRunSucceeded(*devFw, *managedFw, devNamespace, managedNamespace, fbcPreGAAppName, fbcPreGACompName)
			})

			It("verifies release CR completed and set succeeded.", func() {
				assertReleaseCRSucceeded(*devFw, devNamespace, managedNamespace, fbcPreGAAppName, fbcPreGACompName)
			})
		})
	})
})

func assertReleasePipelineRunSucceeded(devFw, managedFw framework.Framework, devNamespace, managedNamespace, fbcAppName string, componentName string) {
	snapshot, err = devFw.AsKubeDeveloper.IntegrationController.WaitForSnapshotToGetCreated("", "", componentName, devNamespace)
	Expect(err).ToNot(HaveOccurred())
	GinkgoWriter.Println("snapshot: ", snapshot.Name)
	Eventually(func() error {
		releaseCR, err = devFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
		if err != nil {
			return err
		}
		GinkgoWriter.Println("Release CR: ", releaseCR.Name)
		return nil
	}, 5*time.Minute, releasecommon.DefaultInterval).Should(Succeed(), "timed out when waiting for Snapshot and Release being created")

	mFw = releasecommon.NewFramework(managedWorkspace)
	// Create a ticker that ticks every 3 minutes
	ticker := time.NewTicker(3 * time.Minute)
	// Schedule the stop of the ticker after 15 minutes
	time.AfterFunc(15*time.Minute, func() {
		ticker.Stop()
		fmt.Println("Stopped executing every 3 minutes.")
	})
	// Run a goroutine to handle the ticker ticks
	go func() {
		for range ticker.C {
			mFw = releasecommon.NewFramework(managedWorkspace)
		}
	}()

	Expect(mFw.AsKubeAdmin.ReleaseController.WaitForReleasePipelineToBeFinished(releaseCR, managedNamespace)).To(Succeed(), fmt.Sprintf("Error when waiting for a release pipelinerun for release %s/%s to finish", releaseCR.GetNamespace(), releaseCR.GetName()))
}

func assertReleaseCRSucceeded(devFw framework.Framework, devNamespace, managedNamespace, fbcAppName string, componentName string) {
	dFw := releasecommon.NewFramework(devWorkspace)
	Eventually(func() error {
		releaseCR, err = dFw.AsKubeDeveloper.ReleaseController.GetRelease("", snapshot.Name, devNamespace)
		if err != nil {
			return err
		}
		conditions := releaseCR.Status.Conditions
		if len(conditions) > 0 {
			for _, c := range conditions {
				if c.Type == "Released" && c.Status == "True" {
					GinkgoWriter.Println("Release CR is released")
				}
			}
		}

		if !releaseCR.IsReleased() {
			return fmt.Errorf("release %s/%s is not marked as finished yet", releaseCR.GetNamespace(), releaseCR.GetName())
		}
		return nil
	}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(Succeed())
}

func createFBCEnterpriseContractPolicy(fbcECPName string, managedFw framework.Framework, devNamespace, managedNamespace string) {
	defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
		Description: "Red Hat's enterprise requirements",
		PublicKey:   "k8s://openshift-pipelines/public-key",
		Sources: []ecp.Source{{
			Name:   "Default",
			Policy: []string{releasecommon.EcPolicyLibPath, releasecommon.EcPolicyReleasePath},
			Data:   []string{releasecommon.EcPolicyDataBundle, releasecommon.EcPolicyDataPath},
		}},
		Configuration: &ecp.EnterpriseContractPolicyConfiguration{
			Exclude: []string{"step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"},
			Include: []string{"@slsa3"},
		},
	}

	_, err := managedFw.AsKubeDeveloper.TektonController.CreateEnterpriseContractPolicy(fbcECPName, managedNamespace, defaultEcPolicySpec)
	Expect(err).NotTo(HaveOccurred())

}

func createFBCReleasePlanAdmission(fbcRPAName string, managedFw framework.Framework, devNamespace, managedNamespace, fbcAppName, fbcECPName, pathInRepoValue, hotfix, issueId, preGA, productName, productVersion string) {
	var err error
	data, err := json.Marshal(map[string]interface{}{
		"fbc": map[string]interface{}{
			"fromIndex":                       constants.FromIndex,
			"targetIndex":                     constants.TargetIndex,
			"binaryImage":                     constants.BinaryImage,
			"publishingCredentials":           "fbc-preview-publishing-credentials",
			"iibServiceConfigSecret":          "iib-preview-services-config",
			"iibOverwriteFromIndexCredential": "iib-overwrite-fromimage-credentials",
			"requestUpdateTimeout":            "1500",
			"buildTimeoutSeconds":             "1500",
			"hotfix":                          hotfix,
			"issueId":                         issueId,
			"preGA":                           preGA,
			"productName":                     productName,
			"productVersion":                  productVersion,
			"allowedPackages":                 []string{"example-operator"},
		},
		"sign": map[string]interface{}{
			"configMapName": "hacbs-signing-pipeline-config-redhatbeta2",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = managedFw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(fbcRPAName, managedNamespace, "", devNamespace, fbcECPName, fbcServiceAccountName, []string{fbcAppName}, true, &tektonutils.PipelineRef{
		Resolver: "git",
		Params: []tektonutils.Param{
			{Name: "url", Value: releasecommon.RelSvcCatalogURL},
			{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
			{Name: "pathInRepo", Value: pathInRepoValue},
		},
	}, &runtime.RawExtension{
		Raw: data,
	})
	Expect(err).NotTo(HaveOccurred())
}
