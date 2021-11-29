//   /**@
//    * This pipeline will deploy the application to the k8s cluster.
//    * 
//    */

// try {
//   node('master') {
//     stage('Checkout') {
//       checkout scm
//     }
//      stage('Build and Push'){
//             docker.withRegistry("", "") {
//                  sh("docker-compose build")
//                  sh("docker-compose push")
//             }
//         }

//   }
// }

// /**
//  */
// catch (exc) {
//   node('master') {
 
//   }
// }
pipeline {
  environment {
    registry = "askljd23084/flask-webhook"
    registryCredential = 'dockerhub_id'
    dockerImage = ''
  }
  agent any
  stages {
    stage('Cloning our Git') {
      steps {
        checkout scm
      }
    }
    stage('Building our image') {
      steps {
        script {
          dockerImage = docker.build registry + ":$BUILD_NUMBER"
        }
      }
    }
    stage('Deploy our image') {
      steps {
        script {
          docker.withRegistry('', registryCredential) {
            dockerImage.push()
          }
        }
      }
    }
    stage('Cleaning up') {
      steps {
        sh "docker rmi $registry:$BUILD_NUMBER"
      }
    }
  }
}