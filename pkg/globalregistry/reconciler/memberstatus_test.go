/*
   Copyright 2021 The Kubermatic Kubernetes Platform contributors.

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

package reconciler_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/kubermatic-labs/registryman/pkg/globalregistry/reconciler"
)

var (
	alpha = reconciler.MemberStatus{
		Name: "alpha",
		Type: "type",
		Role: "role",
	}
	alphaPrime = reconciler.MemberStatus{
		Name: "alpha",
		Type: "otherType",
		Role: "role",
	}
	beta = reconciler.MemberStatus{
		Name: "beta",
		Type: "type",
		Role: "role",
	}
)

var _ = Describe("Memberstatus", func() {
	It("returns no action for the same MemberStatus slice", func() {
		act := []reconciler.MemberStatus{}
		exp := []reconciler.MemberStatus{}
		actions := reconciler.CompareMemberStatuses("proj", act, exp)
		Expect(actions).ToNot(BeNil())
		Expect(len(actions)).To(Equal(0))

		act = []reconciler.MemberStatus{
			alpha,
		}
		exp = []reconciler.MemberStatus{
			alpha,
		}
		actions = reconciler.CompareMemberStatuses("proj", act, exp)
		Expect(actions).ToNot(BeNil())
		Expect(len(actions)).To(Equal(0))

		act = []reconciler.MemberStatus{
			alpha,
			beta,
		}
		exp = []reconciler.MemberStatus{
			alpha,
			beta,
		}
		actions = reconciler.CompareMemberStatuses("proj", act, exp)
		Expect(actions).ToNot(BeNil())
		Expect(len(actions)).To(Equal(0))

		act = []reconciler.MemberStatus{
			beta,
			alpha,
		}
		exp = []reconciler.MemberStatus{
			alpha,
			beta,
		}
		actions = reconciler.CompareMemberStatuses("proj", act, exp)
		Expect(actions).ToNot(BeNil())
		Expect(len(actions)).To(Equal(0))
	})

	It("can detect missing users", func() {
		act := []reconciler.MemberStatus{}
		exp := []reconciler.MemberStatus{
			alpha,
		}
		actions := reconciler.CompareMemberStatuses("proj", act, exp)
		Expect(actions).ToNot(BeNil())
		Expect(len(actions)).To(Equal(1))
		Expect(actionsToStrings(actions)).To(Equal([]string{
			"adding member alpha to proj",
		}))

		act = []reconciler.MemberStatus{
			beta,
		}
		exp = []reconciler.MemberStatus{
			alpha,
			beta,
		}
		actions = reconciler.CompareMemberStatuses("proj", act, exp)
		Expect(actions).ToNot(BeNil())
		Expect(len(actions)).To(Equal(1))
		Expect(actionsToStrings(actions)).To(Equal([]string{
			"adding member alpha to proj",
		}))
	})

	It("can detect surplus users", func() {
		act := []reconciler.MemberStatus{
			alpha,
		}
		exp := []reconciler.MemberStatus{}
		actions := reconciler.CompareMemberStatuses("proj", act, exp)
		Expect(actions).ToNot(BeNil())
		Expect(len(actions)).To(Equal(1))
		Expect(actionsToStrings(actions)).To(Equal([]string{
			"removing member alpha from proj",
		}))

		act = []reconciler.MemberStatus{
			alpha,
			beta,
		}
		exp = []reconciler.MemberStatus{
			beta,
		}
		actions = reconciler.CompareMemberStatuses("proj", act, exp)
		Expect(actions).ToNot(BeNil())
		Expect(len(actions)).To(Equal(1))
		Expect(actionsToStrings(actions)).To(Equal([]string{
			"removing member alpha from proj",
		}))
	})

	It("can detect different users", func() {
		act := []reconciler.MemberStatus{
			alpha,
		}
		exp := []reconciler.MemberStatus{
			alphaPrime,
		}
		actions := reconciler.CompareMemberStatuses("proj", act, exp)
		Expect(actions).ToNot(BeNil())
		Expect(len(actions)).To(Equal(2))
		Expect(actionsToStrings(actions)).To(Equal([]string{
			"removing member alpha from proj",
			"adding member alpha to proj",
		}))
	})
})
