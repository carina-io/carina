/*
   Copyright @ 2021 bocloud <fushaosong@beyondcent.com>.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package runners

import (
	"context"
	"github.com/carina-io/carina/utils"
	"net"
	"os"

	"google.golang.org/grpc"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type gRPCServerRunner struct {
	srv            *grpc.Server
	sockFile       string
	leaderElection bool
}

var _ manager.LeaderElectionRunnable = gRPCServerRunner{}

// NewGRPCRunner creates controller-runtime's manager.Runnable for a gRPC server.
// The server will listen on UNIX domain socket at sockFile.
// If leaderElection is true, the server will run only when it is elected as leader.
func NewGRPCRunner(srv *grpc.Server, sockFile string, leaderElection bool) manager.Runnable {
	return gRPCServerRunner{srv, sockFile, leaderElection}
}

// Start implements controller-runtime's manager.Runnable.
func (r gRPCServerRunner) Start(ctx context.Context) error {
	if !utils.FileExists(r.sockFile) {
		_ = os.MkdirAll(r.sockFile, os.ModeSocket)
	}
	_ = os.Remove(r.sockFile)
	lis, err := net.Listen("unix", r.sockFile)
	if err != nil {
		return err
	}

	go r.srv.Serve(lis)
	<-ctx.Done()
	r.srv.GracefulStop()
	return nil
}

// NeedLeaderElection implements controller-runtime's manager.LeaderElectionRunnable.
func (r gRPCServerRunner) NeedLeaderElection() bool {
	return r.leaderElection
}
