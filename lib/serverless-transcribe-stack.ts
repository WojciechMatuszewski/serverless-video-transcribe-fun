import * as cdk from "@aws-cdk/core";
import * as s3 from "@aws-cdk/aws-s3";
import * as s3Notifications from "@aws-cdk/aws-s3-notifications";
import * as efs from "@aws-cdk/aws-efs";
import * as dataSync from "@aws-cdk/aws-datasync";
import * as ec2 from "@aws-cdk/aws-ec2";
import * as iam from "@aws-cdk/aws-iam";
import * as lambdaGo from "@aws-cdk/aws-lambda-go";
import * as lambda from "@aws-cdk/aws-lambda";
import * as apigwv2 from "@aws-cdk/aws-apigatewayv2";
import * as apigwv2Integrations from "@aws-cdk/aws-apigatewayv2-integrations";
import * as lambdaDestinations from "@aws-cdk/aws-lambda-destinations";
import * as events from "@aws-cdk/aws-events";
import * as eventTargets from "@aws-cdk/aws-events-targets";
import * as sfn from "@aws-cdk/aws-stepfunctions";
import * as sfnTasks from "@aws-cdk/aws-stepfunctions-tasks";
import * as glue from "@aws-cdk/aws-glue";

import { join } from "path";

export class ServerlessTranscribeStack extends cdk.Stack {
  constructor(scope: cdk.Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const eventBus = new events.EventBus(this, "eventBus");

    const dataBucket = new s3.Bucket(this, "dataBucket", {
      accessControl: s3.BucketAccessControl.PRIVATE,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      removalPolicy: cdk.RemovalPolicy.DESTROY
    });

    const createPresignedUrlFunction = new lambdaGo.GoFunction(
      this,
      "createPresignedUrlFunction",
      {
        entry: join(__dirname, "../src/create-presigned-url"),
        environment: {
          BUCKET_NAME: dataBucket.bucketName
        }
      }
    );
    dataBucket.grantPut(createPresignedUrlFunction);

    const efsVpc = new ec2.Vpc(this, "vpc", {
      subnetConfiguration: [
        { name: "public", subnetType: ec2.SubnetType.PUBLIC }
      ]
    });

    efsVpc.addGatewayEndpoint("s3-gateway", {
      service: ec2.GatewayVpcEndpointAwsService.S3
    });

    const fileSystemSecurityGroup = new ec2.SecurityGroup(
      this,
      "fileSystemSecurityGroup",
      {
        vpc: efsVpc,
        allowAllOutbound: true
      }
    );

    const fileSystem = new efs.FileSystem(this, "fileSystem", {
      vpc: efsVpc,
      securityGroup: fileSystemSecurityGroup,
      removalPolicy: cdk.RemovalPolicy.DESTROY
    });

    const fileSystemAccessPoint = fileSystem.addAccessPoint(
      "firstAccessPoint4",
      {
        createAcl: {
          ownerGid: "1001",
          ownerUid: "1001",
          permissions: "0777"
        },
        posixUser: {
          gid: "1001",
          uid: "1001"
        },
        path: "/videos"
      }
    );

    const downloadToEFSFunction = new lambdaGo.GoFunction(
      this,
      "downloadToEFS",
      {
        entry: join(__dirname, "../src/download-to-efs"),
        vpc: efsVpc,
        allowPublicSubnet: true,
        filesystem: lambda.FileSystem.fromEfsAccessPoint(
          fileSystemAccessPoint,
          "/mnt/videos"
        ),
        onSuccess: new lambdaDestinations.EventBridgeDestination(eventBus),
        retryAttempts: 0,
        timeout: cdk.Duration.seconds(20),
        memorySize: 3062
      }
    );

    downloadToEFSFunction.addToRolePolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        actions: ["s3:HeadObject"],
        resources: [dataBucket.arnForObjects("*")]
      })
    );
    dataBucket.grantRead(downloadToEFSFunction);

    dataBucket.addEventNotification(
      s3.EventType.OBJECT_CREATED,
      new s3Notifications.LambdaDestination(downloadToEFSFunction),
      {
        prefix: "input/",
        suffix: "mp4"
      }
    );

    const onFileReadyRule = new events.Rule(this, "onFileReady", {
      eventBus,
      eventPattern: {
        source: ["lambda"]
      }
    });

    const ffmpegLayer = new lambda.LayerVersion(this, "ffmpegLayer", {
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      code: lambda.Code.fromAsset(join(__dirname, "../layer/bin"))
    });

    const createChunksFunction = new lambdaGo.GoFunction(
      this,
      "createChunksFunction",
      {
        entry: join(__dirname, "../src/create-chunks"),
        vpc: efsVpc,
        allowPublicSubnet: true,
        filesystem: lambda.FileSystem.fromEfsAccessPoint(
          fileSystemAccessPoint,
          "/mnt/videos"
        ),
        retryAttempts: 0,
        layers: [ffmpegLayer],
        memorySize: 2048,
        timeout: cdk.Duration.seconds(10),
        environment: {
          CHUNK_SIZE_SECONDS: "30"
        }
      }
    );

    const createChunksTask = new sfnTasks.LambdaInvoke(this, "createChunks", {
      lambdaFunction: createChunksFunction,
      payloadResponseOnly: true,
      resultPath: "$.chunks"
    });

    const parseEvent = new sfn.Pass(this, "parseEvent", {
      parameters: {
        filePath: sfn.JsonPath.stringAt(
          "States.Format('/mnt/videos/{}', $.detail.requestPayload.Records[0].s3.object.key)"
        )
      },
      outputPath: "$"
    });

    const iterateOverChunks = new sfn.Map(this, "chunksIterator", {
      inputPath: "$",
      itemsPath: sfn.JsonPath.stringAt("$.chunks"),
      maxConcurrency: 5,
      parameters: {
        chunk: sfn.JsonPath.stringAt("$$.Map.Item.Value"),
        originFilePath: sfn.JsonPath.stringAt("$.filePath"),
        s3OutputDirectory: sfn.JsonPath.stringAt(
          "States.Format('chunks/{}', $$.Execution.Name)"
        ),
        efsOutputDirectory: sfn.JsonPath.stringAt(
          "States.Format('/mnt/videos/chunks/{}', $$.Execution.Name)"
        ),
        outputFileName: sfn.JsonPath.stringAt(
          "States.Format('{}.mp4', $$.Map.Item.Index)"
        )
      },
      resultPath: sfn.JsonPath.DISCARD
    });

    const splitVideoFunction = new lambdaGo.GoFunction(
      this,
      "splitVideoFunction",
      {
        entry: join(__dirname, "../src/split-video"),
        vpc: efsVpc,
        allowPublicSubnet: true,
        filesystem: lambda.FileSystem.fromEfsAccessPoint(
          fileSystemAccessPoint,
          "/mnt/videos"
        ),
        retryAttempts: 0,
        layers: [ffmpegLayer],
        memorySize: 2048,
        timeout: cdk.Duration.seconds(20)
      }
    );
    const splitVideoTask = new sfnTasks.LambdaInvoke(this, "splitVideo", {
      lambdaFunction: splitVideoFunction,
      payloadResponseOnly: true,
      resultPath: sfn.JsonPath.DISCARD
    });

    const uploadToS3Function = new lambdaGo.GoFunction(
      this,
      "uploadToS3Function",
      {
        entry: join(__dirname, "../src/upload-to-s3"),
        vpc: efsVpc,
        allowPublicSubnet: true,
        filesystem: lambda.FileSystem.fromEfsAccessPoint(
          fileSystemAccessPoint,
          "/mnt/videos"
        ),
        retryAttempts: 0,
        layers: [ffmpegLayer],
        memorySize: 2048,
        timeout: cdk.Duration.seconds(20),
        environment: {
          BUCKET_NAME: dataBucket.bucketName
        }
      }
    );
    dataBucket.grantWrite(uploadToS3Function);
    const uploadToS3Task = new sfnTasks.LambdaInvoke(this, "uploadToS3", {
      lambdaFunction: uploadToS3Function,
      payloadResponseOnly: true,
      resultPath: sfn.JsonPath.DISCARD
    });

    const runTranscribeTask = new sfn.CustomState(this, "runTranscribe", {
      stateJson: {
        Type: "Task",
        Resource: "arn:aws:states:::aws-sdk:transcribe:startTranscriptionJob",
        Parameters: {
          Media: {
            "MediaFileUri.$": sfn.JsonPath.stringAt(
              `States.Format('s3://${dataBucket.bucketName}/{}/{}', $.s3OutputDirectory, $.outputFileName)`
            )
          },
          OutputBucketName: dataBucket.bucketName,
          "OutputKey.$": sfn.JsonPath.stringAt(
            "States.Format('subtitles/{}/{}.json', $$.Execution.Name, $.outputFileName)"
          ),
          "TranscriptionJobName.$": sfn.JsonPath.stringAt(
            "States.Format('{}_{}', $$.Execution.Name, $.outputFileName)"
          ),
          LanguageCode: "en-GB"
        }
      }
    });

    const chunkWorker = splitVideoTask
      .next(uploadToS3Task)
      .next(runTranscribeTask);

    const checkTranscribeFunction = new lambdaGo.GoFunction(
      this,
      "checkTranscribeFunction",
      {
        entry: join(__dirname, "../src/check-transcribe"),
        retryAttempts: 0,
        memorySize: 2048,
        timeout: cdk.Duration.seconds(20)
      }
    );
    checkTranscribeFunction.addToRolePolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        actions: ["transcribe:ListTranscriptionJobs"],
        resources: ["*"]
      })
    );
    const checkTranscribeTask = new sfnTasks.LambdaInvoke(
      this,
      "checkTranscribe",
      {
        lambdaFunction: checkTranscribeFunction,
        payloadResponseOnly: true,
        resultPath: "$.isTranscriptionReady",
        payload: sfn.TaskInput.fromObject({
          executionName: sfn.JsonPath.stringAt("$$.Execution.Name")
        })
      }
    );

    const sleepBetweenChecks = new sfn.Wait(this, "sleepBetweenChecks", {
      time: sfn.WaitTime.duration(cdk.Duration.seconds(30))
    });

    const glueDatabase = new glue.Database(this, "subtitlesDatabase", {
      /**
       * The name cannot contain uppercase characters.
       */
      databaseName: "subtitlesdatabase"
    });
    const glueTable = new glue.Table(this, "subtitlesTable", {
      database: glueDatabase,
      /**
       * The name cannot contain uppercase characters.
       */
      tableName: "subtitlestable",
      columns: [
        {
          name: "jobName",
          type: glue.Schema.STRING
        },
        {
          name: "results",
          type: glue.Schema.array(
            glue.Schema.struct([
              {
                name: "transcripts",
                type: glue.Schema.array(
                  glue.Schema.struct([
                    {
                      name: "transcript",
                      type: glue.Schema.STRING
                    }
                  ])
                )
              }
            ])
          )
        }
      ],
      dataFormat: glue.DataFormat.JSON,
      bucket: dataBucket,
      s3Prefix: "subtitles/"
    });
    /**
     * com.amazonaws.services.s3.model.AmazonS3Exception: Access Denied (Service: Amazon S3; Status Code: 403; Error Code: AccessDenied; Request ID: 7CKGCMSTYH4SSGSB; S3 Extended Request ID: vR79Ne+vHE0VgND/YWh/k/iwnhn9dtytzqJFYDtC0fHqrgmCFoohG5v2tqCPgQYbS6/okVKPFpU=; Proxy: null), S3 Extended Request ID: vR79Ne+vHE0VgND/YWh/k/iwnhn9dtytzqJFYDtC0fHqrgmCFoohG5v2tqCPgQYbS6/okVKPFpU= (Path: s3://serverlesstranscribestack3-databucketd8691f4e-1uvmls64s9fxx/subtitles)
     */
    const concatSubtitlesTask = new sfn.CustomState(this, "concatSubtitles", {
      stateJson: {
        Type: "Task",
        Resource: "arn:aws:states:::athena:startQueryExecution.sync",
        Parameters: {
          "ClientRequestToken.$": "$$.Execution.Name",
          "QueryString.$":
            "States.Format('select results[1].transcripts[1].transcript from subtitlestable WHERE jobName LIKE \\'{}_%.mp4\\' ORDER BY jobName', $$.Execution.Name)",
          ResultConfiguration: {
            "OutputLocation.$": `States.Format('s3://${dataBucket.bucketName}/subtitles/{}/result/', $$.Execution.Name)`
          },
          QueryExecutionContext: {
            Database: glueDatabase.databaseName
          }
        }
      }
    });

    const isTranscriptionDoneChoice = new sfn.Choice(
      this,
      "isTranscriptionDone"
    )
      .when(
        sfn.Condition.booleanEquals("$.isTranscriptionReady", false),
        sleepBetweenChecks
      )
      .otherwise(concatSubtitlesTask);

    const waitForTranscribeLoop = sleepBetweenChecks
      .next(checkTranscribeTask)
      .next(isTranscriptionDoneChoice);

    const stateMachineDefinition = parseEvent
      .next(createChunksTask)
      .next(iterateOverChunks.iterator(chunkWorker))
      .next(waitForTranscribeLoop);

    const stateMachine = new sfn.StateMachine(this, "stateMachine", {
      definition: stateMachineDefinition
    });

    stateMachine.addToRolePolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        actions: ["transcribe:StartTranscriptionJob"],
        resources: ["*"]
      })
    );

    stateMachine.addToRolePolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        resources: [dataBucket.arnForObjects("subtitles/*")],
        actions: ["s3:PutObject"]
      })
    );

    stateMachine.addToRolePolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        resources: [dataBucket.arnForObjects("chunks/*")],
        actions: ["s3:GetObject"]
      })
    );

    stateMachine.addToRolePolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        resources: ["*"],
        actions: ["athena:StartQueryExecution", "athena:GetQueryExecution"]
      })
    );
    stateMachine.addToRolePolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        resources: [
          glueDatabase.catalogArn,
          glueTable.tableArn,
          glueDatabase.databaseArn
        ],
        actions: ["glue:GetTable", "glue:GetPartitions"]
      })
    );
    stateMachine.addToRolePolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        resources: [dataBucket.bucketArn],
        actions: ["s3:GetBucketLocation"]
      })
    );
    stateMachine.addToRolePolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        resources: [dataBucket.bucketArn],
        actions: [
          "s3:GetBucket",
          "s3:ListBucketMultipartUploads",
          "s3:ListMultipartUploadParts",
          "s3:AbortMultipartUpload"
        ]
      })
    );

    onFileReadyRule.addTarget(new eventTargets.SfnStateMachine(stateMachine));

    const api = new apigwv2.HttpApi(this, "api");

    const createPresignedUrlIntegration =
      new apigwv2Integrations.LambdaProxyIntegration({
        handler: createPresignedUrlFunction
      });

    api.addRoutes({
      path: "/",
      methods: [apigwv2.HttpMethod.GET],
      integration: createPresignedUrlIntegration
    });

    new cdk.CfnOutput(this, "apiUrl", {
      value: api.apiEndpoint
    });

    new cdk.CfnOutput(this, "bucketName", {
      value: dataBucket.bucketName
    });
  }
}
