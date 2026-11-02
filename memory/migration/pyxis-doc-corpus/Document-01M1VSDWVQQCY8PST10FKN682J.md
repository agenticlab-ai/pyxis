# Pyxis authored-document memory import manifest

- Watermark: `0a05dfb7e55536efea9452733b767bde2050e6ab`
- Authored source documents: 706
- Source tree-manifest SHA-256: `77e125c3119ab73a097abde171a1ae544127cdd488dc03cbc36ed59ab70011ef`
- Accounted document-like tree entries: 1108
- Accounted document-like tree SHA-256: `cf7fd92038ab8fd917e0ab3f017f3a1147c1e930767645c9ee0d673665f4a099`
- Source bytes were read with `git show <watermark>:<path>`, never from the working tree.
- Document-like means tracked `.md`, `.markdown`, `.yaml`, `.yml`, `.json`, or `.env` entries; every such entry is in exactly one inventory below.
- Session logs are imported as historical, non-governing evidence.

## Closed tree accounting

- `imported`: 706 · SHA-256 `77e125c3119ab73a097abde171a1ae544127cdd488dc03cbc36ed59ab70011ef`
- `generated-derived`: 110 · SHA-256 `7b147948f7585c063a454243ba3f959e0c02fa1f50456e71f0ecdb036df9c4b6`
- `alias`: 1 · SHA-256 `b872bbf25e6ed87fd12214ae21af28f67352b92cd45a5fee5a96fd9a5b4b142c`
- `fixture-testdata`: 267 · SHA-256 `639a36400d61313d2acd11a3a179f53f6469fefa6b1f94e8d7a17c762339a719`
- `mutable-operational-executable-configuration`: 24 · SHA-256 `056703e5e47b4e67efd5e585b4470f4ece95f06521f9ac0a224a76c36c872a5f`

## Known upstream source gaps

- The configured source `docs/system-design.md` does not exist at this watermark.
- No approved memory-specific PRD ledger or technical design exists at this watermark; the imported memory implementation therefore has no approved feature contract of those classes.

## Class counts

- `.pyxis/authoring`: 17
- `.pyxis/class-policy.yaml`: 1
- `.pyxis/config.yaml`: 1
- `.pyxis/context-pack.yaml`: 1
- `.pyxis/dist`: 7
- `.pyxis/model-routing.md`: 1
- `.pyxis/pipeline.md`: 1
- `.pyxis/scripts`: 1
- `.pyxis/severity-map.yaml`: 1
- `.pyxis/skills`: 30
- `.pyxis/stamp`: 2
- `.pyxis/templates`: 19
- `.pyxis/vendor`: 3
- `.pyxis/writehook`: 2
- `cmd/memory`: 1
- `docs/brainstorms`: 63
- `docs/feedback`: 1
- `docs/ideas`: 1
- `docs/learnings`: 1
- `docs/observability`: 1
- `docs/plans`: 33
- `docs/policy`: 2
- `docs/prds`: 107
- `docs/problems`: 10
- `docs/research`: 3
- `docs/reviews`: 107
- `docs/root`: 7
- `docs/runbooks`: 2
- `docs/solution-shapes`: 7
- `docs/tech-design`: 84
- `docs/wiki`: 6
- `logs`: 181
- `repo-root`: 2

## Sources

```text
sha256  class  memory-address  source-path
d55a0e5cbf39f713d225441040596bb19f154f91f577defdc27e941134b6b62a  .pyxis/authoring  authoring/argus#guide  .pyxis/authoring/argus.md
93361139e2449b3a589951ad67b07c5dfff61b6694d09b342ee6fc4d6e458f85  .pyxis/authoring  authoring/e2e-design#guide  .pyxis/authoring/e2e-design.md
ae60c7565ddb9c002af59447c20d81c64c46f059d8c85a2c9b491adf7f479c08  .pyxis/authoring  authoring/e2e-plan#guide  .pyxis/authoring/e2e-plan.md
8b4a7fadf96db07509cf07c298e24d7ba161932a92d4320685a3fbbdb273de3e  .pyxis/authoring  authoring/e2e-review#guide  .pyxis/authoring/e2e-review.md
f624e67f7b2d13e2f39b5aa6b422282fcaa32697232027b9fbfe59579c3cd990  .pyxis/authoring  authoring/feature-brainstorm#guide  .pyxis/authoring/feature-brainstorm.md
4aa6b0825dd76c4ae0ab16d3433e74fb80ed4effcaafe568eb31324e263a4443  .pyxis/authoring  authoring/help#guide  .pyxis/authoring/help.md
feb1580f11e8eb99f63e7b3b33841bd1d439a8744769c40a5ee8638cb6650b97  .pyxis/authoring  authoring/impl-review#guide  .pyxis/authoring/impl-review.md
1ef307344d42d39c641a91ebe56257a1f645dbcead403e7689b6725837437a88  .pyxis/authoring  authoring/implement-now#guide  .pyxis/authoring/implement-now.md
f6e0a979a338a4b13107afe1635ff4e8857bfefe7ac2c187d2acf887100dca6c  .pyxis/authoring  authoring/incident#guide  .pyxis/authoring/incident.md
6e98054e5775866eacc03f8c498690fda93af15c619c9365640bc2ccbb2f5784  .pyxis/authoring  authoring/plan-new#guide  .pyxis/authoring/plan-new.md
b46178ae111c7b63204a283e45b19ae6693ceb6085eea0da2f5758e6e355f6e7  .pyxis/authoring  authoring/prd-amend#guide  .pyxis/authoring/prd-amend.md
0f06460abb986d2b1360cb6fdb5eb3ff29713a2c8f2103eda6021c706caa29be  .pyxis/authoring  authoring/prd-new#guide  .pyxis/authoring/prd-new.md
8f38674b662d10af69de5de6dd086fc1f8e91efb33640ae9488217f68ec9f06b  .pyxis/authoring  authoring/prd-review#guide  .pyxis/authoring/prd-review.md
2bfb94222c943a57f6884f21090d3a5646e94d4ac4063f989f05fd2ae21bc9b2  .pyxis/authoring  authoring/summarize#guide  .pyxis/authoring/summarize.md
118b9036de29a9e42d5f1f91a33da37f35eb5a2c6953ba7127c7a2b451279b08  .pyxis/authoring  authoring/tech-design-brainstorm#guide  .pyxis/authoring/tech-design-brainstorm.md
58d1e817c31055ff13200c2f7f40e65598277c6c6505fde6bee05f67b9e3f71a  .pyxis/authoring  authoring/tech-design-new#guide  .pyxis/authoring/tech-design-new.md
43ca4431b8cb838332d8cf623f1f8991e43bd8d06c9f088d0fbe890a9025b485  .pyxis/authoring  authoring/tech-design-review#guide  .pyxis/authoring/tech-design-review.md
02b494fa16ac1b6e67a55d674d684f1f11de24fe3512ae6e795f8ccfb23ea79b  .pyxis/class-policy.yaml  policy/classification#policy  .pyxis/class-policy.yaml
8ddbee0c8bb95f15eacbd1f03a9649d43b55b94bfc0833665c51c6087ceaea4d  .pyxis/config.yaml  configuration/pyxis#config  .pyxis/config.yaml
64b8b81a517aef1947ea94472922f3034c5c860039e49118a0714dc218e6d5ba  .pyxis/context-pack.yaml  configuration/context-pack#config  .pyxis/context-pack.yaml
2fcbe20de407f27d1bb7ad75668821175000dc8f698995e56710328cb818b3f3  .pyxis/dist  distribution/github-workflows-readme#source  .pyxis/dist/github-workflows/README.md
9632e36ad9bc8020e52cad5502ea4214aa58b76d0cffe50ca3b336d9ac89b6ed  .pyxis/dist  distribution/github-workflows-docs-gate#source  .pyxis/dist/github-workflows/docs-gate.yml
3694fca9d45222dcc059ceb3d4bfff28f84523ff03cf1220e6018f1a12532f1b  .pyxis/dist  distribution/operand-hooks-readme#source  .pyxis/dist/operand-hooks/README.md
6bb26938ea8e4c65843ae12cf99955a987c7de0028aee5b8a0499529a2e79626  .pyxis/dist  distribution/source-workflows-readme#source  .pyxis/dist/source-workflows/README.md
296831072eae595c92fd5e69cf800f21d39887b0386ac080bf9eb4afad92ac6a  .pyxis/dist  distribution/source-workflows-installer-e2e#source  .pyxis/dist/source-workflows/installer-e2e.yml
4ba9803c68b2e231576e0c3ce8a4d1cbf3260e483331d18d36b840c6e7d1b33a  .pyxis/dist  distribution/source-workflows-self-host#source  .pyxis/dist/source-workflows/self-host.yml
4fc273465de02e838014b7313e5d2233472d8ef1c1f4106f66c5d59e548cb624  .pyxis/dist  distribution/source-workflows-system-release#source  .pyxis/dist/source-workflows/system-release.yml
18b7c7737e3aaba4c8dee2f9800ce7f9508b2bb2652ed2132631524763d842e2  .pyxis/model-routing.md  policy/model-routing#policy  .pyxis/model-routing.md
e665975caddaea02202dad6afab7f419f6cf85a99e1eb341784044f5598cfb2e  .pyxis/pipeline.md  system/pipeline#map  .pyxis/pipeline.md
36114ef028bdd5e192ce969ccf3e0f989d2eb3c2c536c7cd018941910ed28f72  .pyxis/scripts  configuration/tool-versions#config  .pyxis/scripts/versions.env
cb23161703c8879dfe5ed6fc49a9a98db5bff321b73f0e8f4325080838bb00d6  .pyxis/severity-map.yaml  policy/contract-gate-severity#policy  .pyxis/severity-map.yaml
6ab1c818d15cba74da72475ea1bc3e6f5ca331cf5868d466a8fe6ecb3447f3c9  .pyxis/skills  skill/argus#entry  .pyxis/skills/argus/SKILL.md
2a656133db3a8f91bba81d912b479c332fb98c536f45849f1806b6acf55e0fab  .pyxis/skills  skill/argus#authority  .pyxis/skills/argus/authority-contract.md
c5cdcadadf2e0f253808c145dc08e7f44d04cfd6dd838be304b9b5ded459340b  .pyxis/skills  skill/doc-review#entry  .pyxis/skills/doc-review/SKILL.md
71dee019e860c3205cd8233b14f54bdc3e8637d6b0d3844fd4f9d6326c6c8a73  .pyxis/skills  skill/e2e-design#entry  .pyxis/skills/e2e-design/SKILL.md
657452d3f56f57e34154bbbd8f3af86954881ce5dac30552116c64d42ecd9f05  .pyxis/skills  skill/e2e-plan#entry  .pyxis/skills/e2e-plan/SKILL.md
79b1141a6fe8e66b066d4c7ab9094b0991f51bf9c6eb1786b85635e4e1814622  .pyxis/skills  skill/e2e-review#entry  .pyxis/skills/e2e-review/SKILL.md
dba915eb5421226c5c8893f3ee51ddb3d72cf0bca0e8bb85a5ee7dc9bb57d38c  .pyxis/skills  skill/e2e-review#calibration  .pyxis/skills/e2e-review/calibration.md
0ade61636f0417143c061e6dae5086e2246f999356be7aeffe8dc3fac75a0159  .pyxis/skills  skill/e2e-review#lens-anti-gaming  .pyxis/skills/e2e-review/lenses/anti-gaming.md
22b2c190e3191ae4283239541668f246bece71972e3223201c289db4b88e384f  .pyxis/skills  skill/e2e-review#lens-buildability  .pyxis/skills/e2e-review/lenses/buildability.md
06650f07fef20fb8ccc8850861112e7bb378a5a07c0e63d642f90c37bb9e8e85  .pyxis/skills  skill/e2e-review#lens-coverage  .pyxis/skills/e2e-review/lenses/coverage.md
2b174b13f931ca2016e04941345792cbd3ff9bcbd9d81e55a26cb293f126e2aa  .pyxis/skills  skill/feature-brainstorm#entry  .pyxis/skills/feature-brainstorm/SKILL.md
00067f4e3f3d9b180d88debaa474aaadeba65bf6be80eab7235fa41afd326270  .pyxis/skills  skill/help#entry  .pyxis/skills/help/SKILL.md
f0d662c2b33e80bbd0367379d8d1927b2f5fb6d52887c3c0fcfe383f22109dbd  .pyxis/skills  skill/impl-review#entry  .pyxis/skills/impl-review/SKILL.md
999c6cf021ce021d75a5b4ebc65dc1955d7b5523286a4a0ba4479548c15119d8  .pyxis/skills  skill/impl-review#calibration  .pyxis/skills/impl-review/calibration.md
0184994df0d70e1620088a009562f23740d0f905a85d8770721c4c6825d9777a  .pyxis/skills  skill/impl-review#lens-correctness-critic  .pyxis/skills/impl-review/lenses/correctness-critic.md
219c821455be59dbdd373a003bbe569cde975eecd52c9078b0c1a5e9e5e0e3da  .pyxis/skills  skill/impl-review#lens-design-conformance  .pyxis/skills/impl-review/lenses/design-conformance.md
010cf21e32b07238c0cb12c10790681c57c313daafd6ac9e98236ca688bf12ce  .pyxis/skills  skill/impl-review#lens-security-abuse  .pyxis/skills/impl-review/lenses/security-abuse.md
c701223dc1765d2eefae017c6337bc25367f2dd6a178f4d5aaa6531d1a01ba0b  .pyxis/skills  skill/impl-review#lens-spec-fidelity  .pyxis/skills/impl-review/lenses/spec-fidelity.md
1fee837396c17921f2e9352ee82f6b06a2f6b5b25bd79673ebcf836035fda63b  .pyxis/skills  skill/implement-now#entry  .pyxis/skills/implement-now/SKILL.md
9dd6990b1bd3403c1992ab74f891e7641cef827d9f50d7591084f541d661e4e1  .pyxis/skills  skill/incident#entry  .pyxis/skills/incident/SKILL.md
84da5c3fe17b5c837b48292a0812491980c40d2c6db3395dde4ebd71e6f68862  .pyxis/skills  skill/plan-new#entry  .pyxis/skills/plan-new/SKILL.md
a736b274114a9ca6bc998427fceae7314da234755b805a186c6913c708966532  .pyxis/skills  skill/prd-amend#entry  .pyxis/skills/prd-amend/SKILL.md
661bd93cd2f21c6ea6b15fd7127604fd09450e432ac6a27c56110ede0495f2cd  .pyxis/skills  skill/prd-new#entry  .pyxis/skills/prd-new/SKILL.md
6861fc2219f7899ff9510c64e3d434392a7f067282d477f08858d82b61a5a56a  .pyxis/skills  skill/prd-review#entry  .pyxis/skills/prd-review/SKILL.md
20e3a1c336b11a983e9f04cf4683a5ccb487641b1bd7deaa0f12e4189a743257  .pyxis/skills  skill/prd-review#calibration  .pyxis/skills/prd-review/calibration.md
0d030161840593cc5bcbf3eda21a63bcd3e250386593fd8a465d66ab4c8f9a27  .pyxis/skills  skill/summarize#entry  .pyxis/skills/summarize/SKILL.md
2b5a29ffe653b7f9fe7d43c025da3ebdfacb0d70bee3c79226c9de415515f6d6  .pyxis/skills  skill/tech-design-brainstorm#entry  .pyxis/skills/tech-design-brainstorm/SKILL.md
f1b758fdb2e58ed883bc2fd4882f3528c173fab0142ae8c6922da9f03ef98ee2  .pyxis/skills  skill/tech-design-new#entry  .pyxis/skills/tech-design-new/SKILL.md
a617f61b3d5175ef25449854f8b75277a4d765863f70a1eaea816fcd75a5080e  .pyxis/skills  skill/tech-design-review#entry  .pyxis/skills/tech-design-review/SKILL.md
a41bccee6fb169b55c30c4e331bc84defd19950fb0c4bf6d1a0cc351f22e642d  .pyxis/skills  skill/tech-design-review#calibration  .pyxis/skills/tech-design-review/calibration.md
69778bc01d024bc33cf4e01032e7292c1b1d79d62f5c6bf4fea62e6d9ee0f717  .pyxis/stamp  integration/argus-stamp#overview  .pyxis/stamp/README.md
74575e6d6e3d04fb1f3feb682025bde13359328be0592fdab85affce3941d4ae  .pyxis/stamp  integration/argus-stamp#claude  .pyxis/stamp/claude.md
65b71a01649ba5e3faa8b7d819a268a81e0451c2f9eb7670c96b046aa64aa481  .pyxis/templates  template/architecture-principles-skeleton#template  .pyxis/templates/architecture-principles-skeleton.md
65c7b703d80a0afadfb30fc6cc67f83e1d8807f4b973d29429ec6ab6c4d14693  .pyxis/templates  template/authority-contract-template#template  .pyxis/templates/authority-contract-template.md
e2dbfcd208b3780afac1accbdecb5c09add8505073862c835d11b61e952ac6c5  .pyxis/templates  template/brainstorm-record-template#template  .pyxis/templates/brainstorm-record-template.md
423b1d8a536a3dcbee41ff430f9160677f5dc3d9b31d781782b3d7ff8c5b5a4d  .pyxis/templates  template/delivery-ledger-template#template  .pyxis/templates/delivery-ledger-template.md
5b06114246965616b878a6ace6db889dbc48eed7c74596a05f968dc4dd374f02  .pyxis/templates  template/design-brainstorm-record-template#template  .pyxis/templates/design-brainstorm-record-template.md
8ab9bdeb43ded8ecd7178999a0270d25bfa58115a0b5618b594ee20e3761b79b  .pyxis/templates  template/design-review-report-template#template  .pyxis/templates/design-review-report-template.md
6bfb8c2664c56104c87b222055c2630bac58598bb05bf983c081055ffd715272  .pyxis/templates  template/e2e-design-template#template  .pyxis/templates/e2e-design-template.md
f34332f2e3cb27609790acc4d4f6578c03cd588c74d40510e184bebe711cb432  .pyxis/templates  template/e2e-review-report-template#template  .pyxis/templates/e2e-review-report-template.md
00d1d8221965134bc160b87089a372a1036569b738aa70a92bb41852c5f6ef9b  .pyxis/templates  template/harness-implement-now-skill#template  .pyxis/templates/harness-implement-now-skill.md
b343a54799d4c8cdd424e8b27ab5f0f426844b3c4890d2fb7e3235b6ed537f32  .pyxis/templates  template/incident-entry-template#template  .pyxis/templates/incident-entry-template.md
d17dde54ac5d673141d5aef758ad9418e5a35440ff585f2f0c209981d761949e  .pyxis/templates  template/operand-agents-manual#template  .pyxis/templates/operand-agents-manual.md
e64340504b5466c2e386c44cd99f2d68edfa4426c7c08e4ef7cba31b9bb4f2cd  .pyxis/templates  template/plan-template#template  .pyxis/templates/plan-template.md
2ae0793a6859a7c1287054f9cdd21bafbf65c52ca9e3c9f86e6a2fd7c4983efc  .pyxis/templates  template/prd-amendment-template#template  .pyxis/templates/prd-amendment-template.md
f710f914c3bdde2fe354cb751ef4aba1a29adba606da0c39685299bc7682d7e2  .pyxis/templates  template/prd-review-report-template#template  .pyxis/templates/prd-review-report-template.md
e38eebb983f910ec73c3b0861743937b9e0b26b2344c09cf1518f4ab3ef5c5b8  .pyxis/templates  template/prd-template#template  .pyxis/templates/prd-template.md
2a5cfae7528824b7467fe5ebc41048258159f320ed4d1ce900ffce83989a5765  .pyxis/templates  template/problem-ledger-template#template  .pyxis/templates/problem-ledger-template.md
953d9364de493d8ae019b8b9c022453de06b658bed5e7253074931e94348d3f5  .pyxis/templates  template/product-vision-skeleton#template  .pyxis/templates/product-vision-skeleton.md
a99a45fb5753903eba6e01fefd9ad4ebd71039dd502e640e9d6dd7ddbc33e19d  .pyxis/templates  template/tech-design-template#template  .pyxis/templates/tech-design-template.md
b7b7ed0a00d69334edc2399cd3c6ca5397592c5b371b07d8b02d38160e141d2a  .pyxis/templates  template/welcome-item#template  .pyxis/templates/welcome-item.md
140f19aa41f34bc03d3b2a9ae9916bf4b974f45090b6537b10fefbd4aef1554c  .pyxis/vendor  vendor/lavish-axi#readme  .pyxis/vendor/lavish-axi/README.md
854c063c48419578e594dd855f8cf3cfdbdc1d3446652c97a322c640ee42c12f  .pyxis/vendor  vendor/litellm-prices#readme  .pyxis/vendor/litellm-prices/README.md
d9950999098ac478b1b424e896d5d6677ee740503b3c554b8918cb5fc06188c2  .pyxis/vendor  vendor/mermaid#readme  .pyxis/vendor/mermaid/README.md
c29e0841c0e457f52cc7ea85b46f8a01bf7ca9cb60d104526c79fab86e45e55c  .pyxis/writehook  integration/argus-writehook#overview  .pyxis/writehook/README.md
aff99bf701a0325cb180976e3878aef24b78b2c8efeeb4619070d1de84efee31  .pyxis/writehook  integration/argus-writehook#claude-settings  .pyxis/writehook/claude.settings.json
db0f2e7241a91698fed722368003e99f924cf667917974a7cb2e8a1fdf4469de  repo-root  governance/agent-operating-manual#manual  AGENTS.md
b0b45a6932be940b371c8d518d1402bc07cecb4276b647eb8fbab800c15a5d85  repo-root  project/pyxis#readme  README.md
193420e65934f64e5ff3e5990b8968e5b45898cf8d0c094efa05e23ba5ff540b  cmd/memory  tool/memory#readme  cmd/memory/README.md
5d317ef0a1d2db822ea779344dfa22c243b981d043f002cd8bf9d393c48335bf  docs/root  governance/architecture-principles#principles  docs/architecture-principles.md
b730d7f5e551a2a1640eb906a25bf6ec412279184d708cad42ba5ccce9ff1198  docs/root  governance/argus-vision#vision  docs/argus-vision.md
fcf77b4ebc2d5fa56346fb4af5f5969e07492eaafaceb6ffaad6a6b75caf45c8  docs/brainstorms  brainstorm/argus-attribution-hardening-tech-design#record  docs/brainstorms/argus-attribution-hardening-tech-design.md
2600d7e188ff4a240cdfd9aabf4aec7c6da1f9b646424f18915c2d0a13d30da1  docs/brainstorms  brainstorm/argus-attribution-hardening#record  docs/brainstorms/argus-attribution-hardening.md
ae072abcec1e6a15a318f47233b8f02900431c1091a0e89d7db429457731eace  docs/brainstorms  brainstorm/argus-boundary-enforcement-tech-design#record  docs/brainstorms/argus-boundary-enforcement-tech-design.md
2d87f46a8dc4df88ec82a0950581398c4964d14d6233d77e28c497cfddf11b44  docs/brainstorms  brainstorm/argus-boundary-enforcement#record  docs/brainstorms/argus-boundary-enforcement.md
ce64e08e6a619a295b57dc2596fbbd315a1a58093ee085e58f5f690171ad2884  docs/brainstorms  brainstorm/argus-ledger-lint-tech-design#record  docs/brainstorms/argus-ledger-lint-tech-design.md
39279f8acd37383a523ecee1d05e736bc5a189afe58ee0013747995563cc41f7  docs/brainstorms  brainstorm/argus-ledger-lint#record  docs/brainstorms/argus-ledger-lint.md
ce40dbb48ad80408df5329780dd9e1a5fcf13895f121a2ff2b2689145cd70ea1  docs/brainstorms  brainstorm/argus-tech-design#record  docs/brainstorms/argus-tech-design.md
2ba837be7ea09a0dddcec28e105b1046ff07741087431bfbb5cf174b86969373  docs/brainstorms  brainstorm/argus-unattended-drive-tech-design#record  docs/brainstorms/argus-unattended-drive-tech-design.md
0eb2a0e24d221f58fb979618517699883e35784eaf20f8de2a9f9ffe674a394a  docs/brainstorms  brainstorm/argus-unattended-drive#record  docs/brainstorms/argus-unattended-drive.md
a19c1036fd39cb609585907caf8344e0d5e061a76bfccf2bd7c403cad78cad6f  docs/brainstorms  brainstorm/authoring-reviewers#record  docs/brainstorms/authoring-reviewers.md
c05774894b750250eba84e96f3241f81a371ed11634179fd6fbfd57101252f32  docs/brainstorms  brainstorm/autonomous-developer#record  docs/brainstorms/autonomous-developer.md
a793a9d33638172a06fc7a428bd1637a82e3e03721e193d5d33eaa46c9f78f34  docs/brainstorms  brainstorm/channel-inbox#record  docs/brainstorms/channel-inbox.md
2eeda82889353fc70bde4676a0c914f4c6f5503adfaee918de1b160fa5ba9d26  docs/brainstorms  brainstorm/code-gate-mirror-tech-design#record  docs/brainstorms/code-gate-mirror-tech-design.md
a4cc15d79a5c9608b8f8888832a1ac4c44b54e21934dc5720a1a86b1e3dc6cfd  docs/brainstorms  brainstorm/config-driven-portability-tech-design#record  docs/brainstorms/config-driven-portability-tech-design.md
db2383c8ccea14e63e00b085a67bb38b9687461ed7cde6557f4b6c66ab459f9e  docs/brainstorms  brainstorm/config-driven-portability#record  docs/brainstorms/config-driven-portability.md
96955cadc89437800565d1a1e2ebb0ab89878b79da90822a80c5e1e2605bb9f6  docs/brainstorms  brainstorm/contract-compiler-2#record  docs/brainstorms/contract-compiler-2.md
526c59a26638015efb4c91a44d44e8a49842b4ad8a516bc069a19ff282d3fba0  docs/brainstorms  brainstorm/contract-compiler#record  docs/brainstorms/contract-compiler.md
08dfab9fef806238c32d90f7d43230257b90c043e768c3b8d8f4df7270ef1acc  docs/brainstorms  brainstorm/contract-gate-tech-design#record  docs/brainstorms/contract-gate-tech-design.md
b33f045e9351ce6a5a110fadad83f91abb36596b5a40b4bb3d005ad90640a580  docs/brainstorms  brainstorm/e2e-verification-tech-design-2#record  docs/brainstorms/e2e-verification-tech-design-2.md
d621aeca1eaf1377d0ac4873ea015b199d435a0d1f91f48209f6c1c54b459ed0  docs/brainstorms  brainstorm/e2e-verification-tech-design#record  docs/brainstorms/e2e-verification-tech-design.md
fd72362e4271fe6f98b71b48550d7c1c0c50c4f8316bc1af9205d0fe7872d63e  docs/brainstorms  brainstorm/e2e-verification#record  docs/brainstorms/e2e-verification.md
f30db156d831dfe65a036dad0273f07e4cc6b242bec3763b0004ac52edfe38b7  docs/brainstorms  brainstorm/ecosystem-help-tech-design#record  docs/brainstorms/ecosystem-help-tech-design.md
c510a4e34a8a2e158ce27c4a89bb57a7ff961fd29c64794e723c55a3940a2427  docs/brainstorms  brainstorm/ecosystem-help#record  docs/brainstorms/ecosystem-help.md
5f688e4f69c216054e340b16a6d1633ee08315d127857f766d29205216e15c9c  docs/brainstorms  brainstorm/existing-repo-onboarding-tech-design#record  docs/brainstorms/existing-repo-onboarding-tech-design.md
f593b79c737fa9d5dd4ce5143524e9c225b8eb38c70f5316d5d9f20a6bc5c944  docs/brainstorms  brainstorm/existing-repo-onboarding#record  docs/brainstorms/existing-repo-onboarding.md
1899759bcca6c289003330d7d925d91cc9c692106ebef861c226467f08e1eb8d  docs/brainstorms  brainstorm/feature-implementation-drive-tech-design#record  docs/brainstorms/feature-implementation-drive-tech-design.md
924afece0789802c668a94f192a620d6dbba7f69ad7848a038be1e8165c9c5c6  docs/brainstorms  brainstorm/feature-implementation-drive#record  docs/brainstorms/feature-implementation-drive.md
b924199a95ddef81f5a8358f2c2efeadfbd8d8db96014a95533ae9a548b9f8d7  docs/brainstorms  brainstorm/gate-attention#record  docs/brainstorms/gate-attention.md
e7486fe664e373bbf11c56b122324e51baa4a7bbb08576954e2ab3f89026a980  docs/brainstorms  brainstorm/impl-review-tech-design#record  docs/brainstorms/impl-review-tech-design.md
25cfac3ba70ce250ae913e716db5d195a05e0ca0a14b10e1da4c9ea6c8c6c5fb  docs/brainstorms  brainstorm/impl-review#record  docs/brainstorms/impl-review.md
51b72ebeb3f1db393bf6dcb15d72d311ec02b1a1fc15f0f6d87c4124bd629c21  docs/brainstorms  brainstorm/implement-now-tech-design-2#record  docs/brainstorms/implement-now-tech-design-2.md
3f1ab4ccf5e53dedb3eeb11f5520188844a7859d6bce6aec01c451a937cfd13d  docs/brainstorms  brainstorm/implement-now-tech-design#record  docs/brainstorms/implement-now-tech-design.md
8df7cd92d04669a6bc42d5d8094f30a8d969f6fa15f738667cde31a123958fa7  docs/brainstorms  brainstorm/implement-now#record  docs/brainstorms/implement-now.md
e46194a64ab17e47584232b150ffd4c05ddfd4e2b3f2800f61c2f641aa1507d4  docs/brainstorms  brainstorm/implementation-verification-tech-design#record  docs/brainstorms/implementation-verification-tech-design.md
cbc0941c0da62ba110dfdd1cf92c7e6feeb2f5031ac996b09b4135ca52430c7f  docs/brainstorms  brainstorm/implementation-verification#record  docs/brainstorms/implementation-verification.md
82f67961f7f3379ce4d40dcbc3750e05001540364b19c6129047aff0d051a871  docs/brainstorms  brainstorm/init-hardening-tech-design#record  docs/brainstorms/init-hardening-tech-design.md
b1d4a0ce2974c15b73d5a07eaef7fc1c33342ae46247e8824aa56513ac9c251c  docs/brainstorms  brainstorm/init-hardening#record  docs/brainstorms/init-hardening.md
6dd7c0b54b20e65d5011f6753f4efd6c8d7bbadd30e2bf89f43a6de85403e9b0  docs/brainstorms  brainstorm/installer-tech-design#record  docs/brainstorms/installer-tech-design.md
afdbfabe229926377e933eef195427af3cdc456e5583e08bc85f0f3327ad517c  docs/brainstorms  brainstorm/installer#record  docs/brainstorms/installer.md
6867b5020457493dff6938d01bbd539fc2d9f22bb7078ecc33f92bd529863172  docs/brainstorms  brainstorm/maintenance-sweep-tech-design#record  docs/brainstorms/maintenance-sweep-tech-design.md
852105836381cc89c54f570de6c42747cec758ef9e0bbd165c43a77bfc7f322d  docs/brainstorms  brainstorm/maintenance-sweep#record  docs/brainstorms/maintenance-sweep.md
9249b765e7716a434f585f1b225fd4e1b181a3c6ed1e02947121db7929a9fea4  docs/brainstorms  brainstorm/pipeline-measurement-tech-design#record  docs/brainstorms/pipeline-measurement-tech-design.md
b849a8085e6fc359a2ddb922ac4078c980500c1538db1965d064b8559082227e  docs/brainstorms  brainstorm/pipeline-measurement#record  docs/brainstorms/pipeline-measurement.md
e1055118fff1eac37f732c97dd7c308230e038fd0412eda1d3a4aa61767a4228  docs/brainstorms  brainstorm/planner-driven-model-routing-tech-design-2#record  docs/brainstorms/planner-driven-model-routing-tech-design-2.md
7a5423dc5a4d3b5ca5aedf5e2c6ee0499796bb5e1f2b6ea345a102ed7383f520  docs/brainstorms  brainstorm/planner-driven-model-routing-tech-design-3#record  docs/brainstorms/planner-driven-model-routing-tech-design-3.md
7a6e126bf9b513a712c648b1f61fa311f877e0333dd5ea852c98f539a309a4a5  docs/brainstorms  brainstorm/planner-driven-model-routing-tech-design-4#record  docs/brainstorms/planner-driven-model-routing-tech-design-4.md
346ecfd7f31afed830838e834e22acae534f7c0ccc11ea7e9be45b69247c31a9  docs/brainstorms  brainstorm/planner-driven-model-routing-tech-design#record  docs/brainstorms/planner-driven-model-routing-tech-design.md
2103c931823c70b58e47c9bf85ecc70eabe7af6ccc7309703b9b3d43a44fcc02  docs/brainstorms  brainstorm/planner-driven-model-routing#record  docs/brainstorms/planner-driven-model-routing.md
e00f2a3a30cf408c9ae38efb20bf3b476184626c55e9384967324fcd2289ed33  docs/brainstorms  brainstorm/read-to-rule-tech-design#record  docs/brainstorms/read-to-rule-tech-design.md
80eac31ef7e27422bf7b589c6bbafe481b23d2002492fca20fa702bb2fd5cc2d  docs/brainstorms  brainstorm/review-feedback-capture-tech-design#record  docs/brainstorms/review-feedback-capture-tech-design.md
27ca0ae000ed9e3737918462632eafbd1ee083d2cd5751054bb3e71183f86c41  docs/brainstorms  brainstorm/review-feedback-capture#record  docs/brainstorms/review-feedback-capture.md
2385aeb6cabb2c42bc3c30989fc0ba58d3d7dd4d4d3bc2459cee8f4ece76953c  docs/brainstorms  brainstorm/reviewer-artifacts-tech-design#record  docs/brainstorms/reviewer-artifacts-tech-design.md
5977370f0182f569b49a9413fe911c3fb652da841271bf4238c934308e0d8a03  docs/brainstorms  brainstorm/self-drive-wake-tech-design#record  docs/brainstorms/self-drive-wake-tech-design.md
252d193fda84272593417fad27eff186c7b28d885a168836d5a5ab633478563d  docs/brainstorms  brainstorm/self-drive-wake#record  docs/brainstorms/self-drive-wake.md
09ea3fd715671a813d960118f623b9df25e96c408f6e73cb14131d2a92f14130  docs/brainstorms  brainstorm/skill-command-prefix-tech-design#record  docs/brainstorms/skill-command-prefix-tech-design.md
fd6e1e0b0d512205c9705d109460c6ba858b2679a5a10243665204a89cab0d70  docs/brainstorms  brainstorm/skill-command-prefix#record  docs/brainstorms/skill-command-prefix.md
3a6dcfc18c78ce67085fac64777517bae0fb3a564205f486605b0e19aff45758  docs/brainstorms  brainstorm/tech-design-brainstorm#record  docs/brainstorms/tech-design-brainstorm.md
e01b2f470b9962be814853a11c6f4d3a7cb6013b76e215f3ec87c85dddaca24f  docs/brainstorms  brainstorm/tech-design-review#record  docs/brainstorms/tech-design-review.md
a55a1ac226b56fe5fc8759495f368a20c0e2c3895a8fa2b05324d729cce64e28  docs/brainstorms  brainstorm/workbench-ui-tech-design#record  docs/brainstorms/workbench-ui-tech-design.md
055f1bae3e4011ccbc29a3cc70d8ba66c3f7b385c6cd465e49db6f2750d6fa2f  docs/brainstorms  brainstorm/workbench-ui#record  docs/brainstorms/workbench-ui.md
222ae463f55b99eb4fea433455dcaa06c0cbd530e653f3905743bf4d5beaf050  docs/brainstorms  brainstorm/worker-transcript-metrics-tech-design#record  docs/brainstorms/worker-transcript-metrics-tech-design.md
caa856245e7eba6abbe28bd59aece8f708729d014c48c52aa8fe47bb7a19ab9f  docs/brainstorms  brainstorm/worker-transcript-metrics#record  docs/brainstorms/worker-transcript-metrics.md
6669cf36d985c8168b410fa55ed4b54d277a091c72764f9155963fbe79f07372  docs/brainstorms  brainstorm/worktracker-hierarchy-query-tech-design#record  docs/brainstorms/worktracker-hierarchy-query-tech-design.md
5377805184bb2aec10bc81e16eb2a74c24e2972cc4385869fa97fd33432860bf  docs/root  governance/architecture-principles#compatibility-pointer  docs/design-principles.md
41766bd20d36296ba9990b5130f62ec85d7beb6141eafd963ce50ca827035883  docs/root  system/domain-model#map  docs/domain-model.md
4fc2066ee6efd9e252525cd73069fe0f7b0e22b1d530b68bd97eadc1c70a63ac  docs/feedback  feedback/002-plan#entry-001-doc-review  docs/feedback/002-plan/001-doc-review.md
c5bcc81b0492d17f292c96b774764e052b918ae2e76e6432005e226a14935a1d  docs/ideas  idea/repo-neutral-governance#record  docs/ideas/repo-neutral-governance.md
9634c2a9e2de37379043d665d8f5061d957c51e12c5b13e5c0ba61e6e06a33f0  docs/learnings  learning/implement-now-codegate-dogfood#record  docs/learnings/implement-now-codegate-dogfood.md
a49dddfe8bdde1cd1d973103abaf087f835ae08534a4971ae84a950dde96f46b  docs/observability  incident/registry#incidents  docs/observability/incidents.md
c4336225525549ce5c8379c8caeae830c7e6316b20d7e2d819ff02e810bd6166  docs/plans  plan/argus-boundary-enforcement#plan-001  docs/plans/argus-boundary-enforcement/001-plan.md
b6bbb72f51b5116e84d3ece9a2a427c23c8d73dc8b28fc946200d21743455e83  docs/plans  plan/argus-boundary-enforcement#plan-002  docs/plans/argus-boundary-enforcement/002-plan.md
bcb26ab6ecdc682e514eee8bd59348e5e69d87e62d5aa137d0c582d70d8c6cd2  docs/plans  plan/argus-ledger-lint#plan-001  docs/plans/argus-ledger-lint/001-plan.md
7e952b5e35d9507d36048ebbfccd31dca6eef6ac9a68e8cafa88073fb1c4fba2  docs/plans  plan/argus-unattended-drive#plan-001  docs/plans/argus-unattended-drive/001-plan.md
3468133700e32d9e272261339607b59a142e22cb88408077b68d5cb56758549e  docs/plans  plan/config-driven-portability#plan-001  docs/plans/config-driven-portability/001-plan.md
98c479ff051c795e28034d9951013b6932e2ab1c44daf92076fb35713cba4be0  docs/plans  plan/context-pack#plan-001  docs/plans/context-pack/001-plan.md
2bc8432d3dcc68d0427992768c4aa73bf7ca275221d5c103d5fcfaec839416b3  docs/plans  plan/contract-gate#plan-001  docs/plans/contract-gate/001-plan.md
a23698f51ca3c7ca69eb775904b5921451abf15d66f8e57c0ecfe70b7dd70f44  docs/plans  plan/contract-gate#source-dogfood-disposition  docs/plans/contract-gate/dogfood-disposition.md
4cb8f844a322374171165bc09642a299a7efeacae2b2ff0616fd7aa3e1bff491  docs/plans  plan/design-solution-shape#plan-001  docs/plans/design-solution-shape/001-plan.md
28806e153188dcd88f4df223033ff3999241c7caa1fe4e99e5780deea06c995e  docs/plans  plan/e2e-verification#plan-001  docs/plans/e2e-verification/001-plan.md
bbca7a4a6b6602a36e7cddf9c149b7538088e7706a708e22c6ef16d4c3081cfa  docs/plans  plan/e2e-verification#plan-002  docs/plans/e2e-verification/002-plan.md
fb6c3d8cb4b170e6b25a1c9e112aaf8be7460fb1851c290d676d11aaa0c10be9  docs/plans  plan/ecosystem-help#plan-001  docs/plans/ecosystem-help/001-plan.md
a752841e089af5e6a8806a0bef3c37cc71c11ddd470cf5f0d64ab2a897b724c1  docs/plans  plan/existing-repo-onboarding#plan-001  docs/plans/existing-repo-onboarding/001-plan.md
70c01f03b937db948d6cef7f50452184ff67e6e2fd97c01fad4ff47c64170041  docs/plans  plan/feature-implementation-drive#plan-001  docs/plans/feature-implementation-drive/001-plan.md
014aff5b7e334320a467775ba84b293cbbab48a3b4c44954aa5fc35b597dd4d1  docs/plans  plan/impl-review#plan-001  docs/plans/impl-review/001-plan.md
1e17c2c37340c4a0258eec72a641e5ad8018bd65788c3df4aea4ae6b56758884  docs/plans  plan/implement-now#plan-001  docs/plans/implement-now/001-plan.md
4c82e6d2d3820c053525e0b94cb007dedbc8cac1721f30480a7f978f7a2438e2  docs/plans  plan/implement-now#plan-002  docs/plans/implement-now/002-plan.md
bd69028ee8153380dca0c2f5a08896c65262610d4d92a570ac6ae41a4efe9ff0  docs/plans  plan/implementation-verification#plan-001  docs/plans/implementation-verification/001-plan.md
2dd9416d380a99c13eac1c1523a4d2ff1dbe16e4dacd51ff449b7bdcf6388e21  docs/plans  plan/init-hardening#plan-001  docs/plans/init-hardening/001-plan.md
afce90c272ff3fb4e005c4875d93771044484f563ffb171ec12932e3f0db2c16  docs/plans  plan/installer#plan-001  docs/plans/installer/001-plan.md
f343952a575f0a042d5d348005be17dd871f4afa26229249e5dc60549bf84e07  docs/plans  plan/local-work-tracker#plan-001  docs/plans/local-work-tracker/001-plan.md
417ecd52f5a24bac58b52d73735d8705c85a71578ea7df1eb33e010b34a5f5e5  docs/plans  plan/local-work-tracker#plan-002  docs/plans/local-work-tracker/002-plan.md
d1cd8c1572077f26989c4ef6efe6b368678e35d4c6d1daa6e79091d9460be834  docs/plans  plan/maintenance-sweep#plan-001  docs/plans/maintenance-sweep/001-plan.md
75549e8f9c187f9d048c7a2c963166d0c3eff7d4f6c3faca511660eec3dfb89e  docs/plans  plan/pipeline-measurement#plan-001  docs/plans/pipeline-measurement/001-plan.md
28eb5dc03c17809b43f3497360caba473128951752204019274df0e92aadb7b4  docs/plans  plan/plan-viewer#plan-001  docs/plans/plan-viewer/001-plan.md
428580a3f72e1d8ffd70cdd43dd21a7de0e3cde2d3bc420165330c7cb54d4928  docs/plans  plan/planner-driven-model-routing#plan-001  docs/plans/planner-driven-model-routing/001-plan.md
60dd89f61187c2d97f7b25cd976287b8eae77f02471e66441fbfa6d2cfa715e6  docs/plans  plan/planner-driven-model-routing#plan-002  docs/plans/planner-driven-model-routing/002-plan.md
579535e0b7face71b752e95c3270d3ac41aad4cc2d5877d40c9f52ebf7719b26  docs/plans  plan/planner-driven-model-routing#plan-003  docs/plans/planner-driven-model-routing/003-plan.md
f6aae2ef58b3f3bbaf27d0f09812ebda10dc87b0063f35a1b108fadfa0da2829  docs/plans  plan/review-feedback-capture#plan-001  docs/plans/review-feedback-capture/001-plan.md
08e34e470799bb332766c2ded59e595e9cb2647d7fcec061e2434095c32296e1  docs/plans  plan/self-drive-wake#plan-001  docs/plans/self-drive-wake/001-plan.md
8cb2806ec1c8507335e43630fa29da208e08fcc62e04d8d34a6fabdc370aa472  docs/plans  plan/self-drive-wake#source-proof-run-001  docs/plans/self-drive-wake/proof-run-001.md
6fd11c04ee4dce50084e0a5d57f7c3b97575146fc5df4410b2524851ae6733d4  docs/plans  plan/skill-command-prefix#plan-001  docs/plans/skill-command-prefix/001-plan.md
a0f044e76b555ef76c889497d74e917979512e84db99adcc4b82bdce4688fa8c  docs/plans  plan/workbench-ui#plan-001  docs/plans/workbench-ui/001-plan.md
42602f1a1eca7e2d9698033af17c794b958e52fa6d62beb8993743ecf1050e80  docs/policy  policy/maintenance-sweep#policy  docs/policy/maintenance-sweep.yaml
9efde7eb06e06d01960b678658aef55af5133b76b2dad3afae9977adf1348e9f  docs/policy  policy/risk-policy#policy  docs/policy/risk-policy.yaml
3925b8c9c20bca03eddafe6a1f182c16582b50f675c90ea0352dd9b98b8bc82d  docs/root  rollup/product-requirements#current  docs/prd.md
a5f0f389df8120c3c9c2ece0b1abe89924823cf03b83a572e8857dfeba3c7542  docs/prds  corpus/prd-index#index  docs/prds/README.md
e673cdc2479543cd6bbc73a0a451a77631a173e78226d10a8685c7714fc78496  docs/prds  prd/argus-boundary-enforcement#base  docs/prds/argus-boundary-enforcement/000-base.md
9d0f30be522a921c1d46195264325bd7718a015221ed293f4e5d6e68b7924357  docs/prds  prd/argus-boundary-enforcement#amendment-001  docs/prds/argus-boundary-enforcement/001-attribution-hardening.md
e1b783d106baa8107a6dd3963f124b911fd1647c6a9d2c161f4829b9a758152e  docs/prds  prd/argus-ledger-lint#base  docs/prds/argus-ledger-lint/000-base.md
56aaae623cfdc1ad6f656c291fba4b7602d507c1ba98933e8d1bbaba9042b6ee  docs/prds  prd/argus-unattended-drive#base  docs/prds/argus-unattended-drive/000-base.md
744052e038aa67dbe1c5b56881c503be8bddbab352eada60a203f93f7d5da506  docs/prds  prd/argus#base  docs/prds/argus/000-base.md
6940a676d346cc55d701892e3f8c2a1346997b87ed79922e8b1866db9496d164  docs/prds  corpus/autonomous-implementation#requirements  docs/prds/autonomous-implementation.md
9e77031a66cb6807deaca4750f1d12e13201ea363a73bc2bbce1c9f6f8a4f278  docs/prds  prd/channel-inbox#base  docs/prds/channel-inbox/000-base.md
d6e96c8a0890d7a8f9a8c46b89d08e4773da5c1d053ff7e47887fd689bd0e9b4  docs/prds  prd/config-driven-portability#base  docs/prds/config-driven-portability/000-base.md
babcc6ab4de60bda563d5c38d82ce0af4ac840bf593b46958a9a31f7f0ca6f18  docs/prds  prd/context-pack#base  docs/prds/context-pack/000-base.md
86734c685e890aa6a80d4cbd3708af1d7f1e6f183e43f95e20884b249a4b9d75  docs/prds  prd/contract-gate#base  docs/prds/contract-gate/000-base.md
4e7e97d2a69411d2b6656da8e48e50ece06b97985d4b1c3af6a52829089bc023  docs/prds  prd/contract-gate#amendment-001  docs/prds/contract-gate/001-currency-compatibility-disposition.md
1c37d735f0a0d27087a6127c3e1b06dbd01619095b385372af2a15bd37cc38de  docs/prds  corpus/design-and-planning#requirements  docs/prds/design-and-planning.md
7fef782fa3ee2e153b466ee373da1ed9eedf21f84db347a4598a17c9a630ba04  docs/prds  prd/design-solution-shape#base  docs/prds/design-solution-shape/000-base.md
50c0fad5ef7c0732235af5e93d45b0528d84cadb133e8241b3e896bb6938e913  docs/prds  prd/docgate#base  docs/prds/docgate/000-base.md
af1951d93221590878eb2ff8bb444d460477cf29aef729b91b9ae807ea836012  docs/prds  prd/docgate#amendment-001  docs/prds/docgate/001-commit-time-guard.md
4e6657a8a4ef02d3ea09ffdce63f7422db34691be658d458e0377d0ec74924ec  docs/prds  prd/docgate#amendment-002  docs/prds/docgate/002-rollup-guard.md
0f37847766464a7eadeebcc1b31d94c75eb9dab6387117c6d3ac0c04aec6e360  docs/prds  prd/docgate#amendment-003  docs/prds/docgate/003-v2-universal-lint.md
afb328bfd59a48e188aab2328d14c8098e89907605b9b860f9001e5a87ce5c2a  docs/prds  prd/docgate#amendment-004  docs/prds/docgate/004-requirement-precedence.md
38b80392a4303317d12b78a6b031cc1ddd19a03e20c36774dca1aea718ed4edb  docs/prds  prd/docgate#amendment-005  docs/prds/docgate/005-design-immutability.md
4968e0e7939737c39b0b3533d717fb1e5f888aa73c75bbf73afd7512333f1474  docs/prds  prd/docgate#amendment-006  docs/prds/docgate/006-governing-fence-completion.md
d31ae89ad57cd5ab88340d4a1c5cf45bb2b4465814790fe044c828c1cff4e38e  docs/prds  prd/docgate#amendment-007  docs/prds/docgate/007-serves-lint-hardening.md
909be89163705cade5485e738d6b6e0b89e82284166c1280b058a0ce53725c1c  docs/prds  prd/docgate#amendment-008  docs/prds/docgate/008-integrity-checks.md
da9e925d6106aa4e865b225f68ceb192604918c5ca86931ef4abb7e54f40610c  docs/prds  prd/docgate#amendment-009  docs/prds/docgate/009-envelope-v2-tracking.md
5891dca498de3455f8c76a350f1397b675dea9fcf619a2fa2bfd6bc488902cc5  docs/prds  prd/docgate#amendment-010  docs/prds/docgate/010-incident-lint-hardening.md
986772b40e23c8a330228cf5bb4cf68b1524d9c411873bb259dcbda50e81637c  docs/prds  prd/docgate#amendment-011  docs/prds/docgate/011-index-coverage.md
13d85bfaf9c677a7c13254dff4c38f0f629a72efb597aca703a3e8096bcbffe2  docs/prds  prd/docgate#amendment-012  docs/prds/docgate/012-single-amendment-per-pr.md
896fe2737fe3de540cd6f7d2fb923e00eca59147c37bb5b4fd819097abdbaa8e  docs/prds  prd/docgate#amendment-013  docs/prds/docgate/013-coverage-on-read.md
dad84acdff94d7205e99d218ccaf6340187b017ebbf33d649e91e9ef3215e4c2  docs/prds  prd/docgate#amendment-014  docs/prds/docgate/014-standalone-docs.md
9d7b291357bf2639f2e28fea3395aa9c8e7d239c991f82e80853dd58ac227f04  docs/prds  prd/docgate#amendment-015  docs/prds/docgate/015-prepush-deletion-awareness.md
441748eb359a9abe968bc5f4d1b16829b8aa79fe35290e86cfa3ec795cac6b50  docs/prds  prd/docgate#amendment-016  docs/prds/docgate/016-content-path-purity.md
62990c3d73b27dfcc1408cd27c2d377d02e712e5082d953aa811101632e9e1eb  docs/prds  prd/docgate#amendment-017  docs/prds/docgate/017-retire-rollup-guard.md
0a9570bba4a9c9465ab0c78ddfa220f779d55e015c24943a5b01a3f25d7ff0a7  docs/prds  prd/docgate#amendment-018  docs/prds/docgate/018-projection-congruence.md
92af28a9fc96fda090b9b66104f1917f633d8bbbf6a19fc48ca428888fa86e50  docs/prds  prd/e2e-verification#base  docs/prds/e2e-verification/000-base.md
6c2bcf3e6ba07ce6ce2169f98d82d72e491544592eb1df226a3e774c8dd32925  docs/prds  prd/e2e-verification#amendment-001  docs/prds/e2e-verification/001-no-surface-stop-and-conditional-harness.md
be3e368b736639223bac2fcc70d6eeb90292d8acdbed58645bcb124b0212d41a  docs/prds  prd/e2e-verification#amendment-002  docs/prds/e2e-verification/002-harness-designed-buildable-and-cost-bounded.md
1de724a6124e7f2d8d900210e5489f4fa71e6af8ba68cd68357915b32beec7e3  docs/prds  prd/ecosystem-help#base  docs/prds/ecosystem-help/000-base.md
9777bc56c6a9c54acd55485a8de9f12f0a9a42e87a632dc470b80565a40774f9  docs/prds  corpus/engineer-workbench#requirements  docs/prds/engineer-workbench.md
450fde03ff09f222a1034ace0c35d631b17618e46cde194e457f5e6e4bfd3cbd  docs/prds  prd/existing-repo-onboarding#base  docs/prds/existing-repo-onboarding/000-base.md
109074a9ee02bd804e14e94db7af969c8e3263a15b4f8204a95fc0a196670568  docs/prds  prd/feature-brainstorm#base  docs/prds/feature-brainstorm/000-base.md
74d90c0aabd73169e370d07df02f981713a20306bb102036213be9c4c2e513b1  docs/prds  prd/feature-brainstorm#amendment-001  docs/prds/feature-brainstorm/001-no-shared-grilling-primitive.md
7950d8c38c5455106da7d0b40787f64326723e70e04f02986673486221880470  docs/prds  prd/feature-brainstorm#amendment-002  docs/prds/feature-brainstorm/002-argus-principal-seam.md
3dc3be810c2cce3a593efc4bf004851b8c75159d0ea62d3559f9b68e607ef2b7  docs/prds  prd/feature-brainstorm#amendment-003  docs/prds/feature-brainstorm/003-question-legibility.md
6b1b552dea7cf9779335a3d1398aeaf4911e69de93f99c131253def30e4660ab  docs/prds  prd/feature-implementation-drive#base  docs/prds/feature-implementation-drive/000-base.md
354e7a4bda1377d5d05702786a91d5b2d492727bfcd5ec06d8b7fd95477a1fe4  docs/prds  corpus/fleet-operations#requirements  docs/prds/fleet-operations.md
1e02c9eae32cfe8652e72f926f0609e67864f966e0d7c01ec00e3a583e093d1d  docs/prds  corpus/governance-and-compliance#requirements  docs/prds/governance-and-compliance.md
e2e34ac5aadf31861d512b506cb8b55ac82906fbff1ab14ffdf95a179891b1f7  docs/prds  corpus/identity-and-security#requirements  docs/prds/identity-and-security.md
e03eaf7a2a52a95171f43d3eea50214ac20859834d2c47119bbfe6a76ab5f115  docs/prds  prd/impl-review#base  docs/prds/impl-review/000-base.md
aa6e509b05fd8802d44785d7f699249a9892498e01bc2eb59312e883d3660fce  docs/prds  prd/impl-review#amendment-001  docs/prds/impl-review/001-nr4-corrective.md
b3e03455a010fb1b387985391b319e1d0c713af40c9c495537c6f1a96ce8b202  docs/prds  prd/impl-review#amendment-002  docs/prds/impl-review/002-consume-review-demand.md
b1349a8634550f1c3c4c73333eca84993f95a29a2e9cb85d761a15ebeab86ff2  docs/prds  prd/impl-review#amendment-003  docs/prds/impl-review/003-append-first-routing-evidence.md
36a7594252ddecfbb7b92e8688bc9150c2c44578e6099d6854bb4157340764bf  docs/prds  prd/impl-review#amendment-004  docs/prds/impl-review/004-host-persisted-review-artifacts.md
e0ca7135f0d5124f8da6a9c2f54e3355a412e03ad2ef1de763431ed5de9b7306  docs/prds  prd/implement-now#base  docs/prds/implement-now/000-base.md
608cbc709b0c348297e899999e628b5ae7da70f963fcc9f897a9cbb1cedd7a9b  docs/prds  prd/implement-now#amendment-001  docs/prds/implement-now/001-consume-execution-demand.md
a52e650062a416dcbe322b4ed4ce7486841af04ec074873a8562a38b7732edeb  docs/prds  prd/implement-now#amendment-002  docs/prds/implement-now/002-consume-e2e-tasks.md
c677266933ca5d52a1fd71c626cbd8756c924d6a3d5ae803b3789c214f783c8b  docs/prds  prd/implement-now#amendment-003  docs/prds/implement-now/003-append-first-routing-evidence.md
1b00b32a26925749cbed61b1c98c2a7f142d2e2737bf99b75d26141f5c429104  docs/prds  prd/implement-now#amendment-004  docs/prds/implement-now/004-observable-impl-review-terminal.md
77623911dc2385cd95102feeaaf1f3b96738c3194391d19e2ab55df1d462c872  docs/prds  prd/implement-now#amendment-005  docs/prds/implement-now/005-authenticated-review-attribution.md
0f5e16b13fa9f107b5ce538f4291e14fb53ba6fc341077fee2f9aa0f00c3b0e5  docs/prds  prd/implement-now#amendment-006  docs/prds/implement-now/006-restore-rollout-requirement.md
9521bb37826e94714e1c88809a6f5645ebe52e930c88762905331563135b1e74  docs/prds  prd/implement-now#amendment-007  docs/prds/implement-now/007-reciprocal-review-publication-terminal.md
bab8780baea735d68b22ba11d30fe6c26f064a89f71172c6fb7d85b76ef1bb27  docs/prds  prd/implementation-verification#base  docs/prds/implementation-verification/000-base.md
4eace099df5d54c21d2679dcbc433c95dd0bf043699a57372d152d19f2d4f479  docs/prds  prd/implementation-verification#amendment-001  docs/prds/implementation-verification/001-codegate-seed-verb.md
82a6e4d6bce3dfa328cccc63df1a7ecb87faa72d2b13f44638ac5b7176597f57  docs/prds  prd/incident-machinery#base  docs/prds/incident-machinery/000-base.md
0079842ff9279bd6e894c41e869af20302dcc77d1616cae912d56906b038430b  docs/prds  prd/init-hardening#base  docs/prds/init-hardening/000-base.md
2cf4a27ec63ff50735df3dbb220aad96b67b31b141c63ef1d21230ac66e99668  docs/prds  prd/init-hardening#amendment-001  docs/prds/init-hardening/001-permission-handoff.md
970ca43f8ff9d23cf7832d918494380b867f12e20418ea5a3bba5e5ba40830ba  docs/prds  prd/installer#base  docs/prds/installer/000-base.md
fcf3f9f8fcd069d811b635197d4a96e74ddb842f953d9a844a1170271ce9b0f2  docs/prds  prd/installer#amendment-001  docs/prds/installer/001-init-hardening-alignment.md
72b309c76e748b5d439c6955ae1924947d4bc8cf88dc58e1523ba716117db3a6  docs/prds  prd/installer#amendment-002  docs/prds/installer/002-one-repository-question.md
8c6f166df42271612949ea4e2449e140a376b019011f808bd58d432d1ed352cf  docs/prds  prd/installer#amendment-003  docs/prds/installer/003-default-distribution-channel.md
4fdf23f3db0679ba1ea015fcff203629b8217344e74f08d4208d0a5124f58315  docs/prds  corpus/knowledge-system#requirements  docs/prds/knowledge-system.md
0e027be0dc1f29c2d70521491961c4da9d2b92f9b06d01339c61a4ec0060a4cd  docs/prds  prd/local-work-tracker#base  docs/prds/local-work-tracker/000-base.md
7694b1c3127d14d408335d6c7662ad8138e4931e791b60c89b735995af9a7815  docs/prds  prd/local-work-tracker#amendment-001  docs/prds/local-work-tracker/001-children-query-and-scale.md
4d361cc7aa73f5bcb13d7407c9af34c58127f773e17de40662dbd82042783945  docs/prds  prd/maintenance-sweep#base  docs/prds/maintenance-sweep/000-base.md
9f6ec72f4c7ab4168b7165236c67cd841092897ad4b5759cc2cad600fb89fcea  docs/prds  prd/maintenance-sweep#amendment-001  docs/prds/maintenance-sweep/001-plan-waivers.md
64c016e6f7c19f0932e9c634a5670c77769dd56c3e32db1aeea207dd39c459f6  docs/prds  corpus/observability-and-spend#requirements  docs/prds/observability-and-spend.md
ddcc767001c750416dadb801d4033e1d3ef7edd51c194df5f5e3ab76c1e18086  docs/prds  prd/pipeline-measurement#base  docs/prds/pipeline-measurement/000-base.md
bd681f379d11f22aca3f2d2653310c0bff3979fe5c950a7166d1296d695f2e38  docs/prds  prd/plan-new#base  docs/prds/plan-new/000-base.md
accaf7ba1c29b469d617f0196c76147f8ea85a2d2ee5ff3fe2d7770c3f28e27d  docs/prds  prd/plan-new#amendment-001  docs/prds/plan-new/001-local-tracker-projection.md
cdb4b4f99c269a2c144f5a7dcabad0462076a36e00818612dd166dec09dfce3c  docs/prds  prd/plan-new#amendment-002  docs/prds/plan-new/002-subset-coverage.md
c6d5b7a0798bf5397b2a1158d667f8d346893ad05163fda6615d5d8afa1d47cb  docs/prds  prd/plan-new#amendment-003  docs/prds/plan-new/003-contract-gate-precondition.md
8f30ad70a97e0073df7a1d1e962da7cfa264eec3cbf2c09d185e092b7b4bf6ec  docs/prds  prd/plan-new#amendment-004  docs/prds/plan-new/004-required-execution-demand.md
b69d6a717bbb9de0ed0c80bd905386421a19ad84540d56438f86e601b158729b  docs/prds  prd/plan-viewer#base  docs/prds/plan-viewer/000-base.md
05f3ebd459d2dc749cb1f6c3b5ad2d6fb3a52c1787558ae00536ea35ac9d7845  docs/prds  prd/planner-driven-model-routing#base  docs/prds/planner-driven-model-routing/000-base.md
9b949cacd02d4d23e901619990f05158ff6900735e239c191d3b2d32ae778faf  docs/prds  prd/planner-driven-model-routing#amendment-001  docs/prds/planner-driven-model-routing/001-remove-no-inference.md
e57246da0fb1de141d98e5f1b3f1e79c08837ac75889d0c50a610fc92206b8ba  docs/prds  prd/planner-driven-model-routing#amendment-002  docs/prds/planner-driven-model-routing/002-universal-review-floor.md
1c080f01cf65eba8ef3f7bd448784368c457789714bc5ef2f3b63ebd380de526  docs/prds  prd/planner-driven-model-routing#amendment-003  docs/prds/planner-driven-model-routing/003-append-first-routing-evidence.md
afc58d7926a616a715fa06fd9692f5bfba3a9009536b62bc403b3409329e14de  docs/prds  prd/prd-review#base  docs/prds/prd-review/000-base.md
cdf5d2476dee1f230a17014f41e01b26b3e658dcb1423421e1efdc0e23b01f93  docs/prds  prd/prd-review#amendment-001  docs/prds/prd-review/001-section-zero-skip.md
4fbad0310585b577944c0cc069143a8af8c011b09307cd1b7b795191dd0ac08c  docs/prds  corpus/quality-and-merge#requirements  docs/prds/quality-and-merge.md
ffa01667d7cb59ca7fdec395344246f0b9ce84009f9d023b11c10321cc9ac6aa  docs/prds  prd/read-to-rule#base  docs/prds/read-to-rule/000-base.md
d32f0a13fd65d868d7e53029d574e88cfb8871611131d1ac3590c6049bbd55ea  docs/prds  prd/review-feedback-capture#base  docs/prds/review-feedback-capture/000-base.md
01653ca4af703244b2ac1640c0acb21770050225665b20d69215201128a40008  docs/prds  prd/review-feedback-capture#amendment-001  docs/prds/review-feedback-capture/001-recut.md
26fdd835a9bf77397788e2a424b599a96d1bb5e768b2e5ef30cf45904352e489  docs/prds  corpus/scaffolding-and-onboarding#requirements  docs/prds/scaffolding-and-onboarding.md
0d2d047e98043ae090d1e4194f3ee7c41bdb608e196a72ec8f8b8ecfb8a8c4f5  docs/prds  prd/self-drive-wake#base  docs/prds/self-drive-wake/000-base.md
373133b8c83323ec062c8a2307645ffd13a26cd6a99ef74f7b160c5931bd6631  docs/prds  prd/self-drive-wake#amendment-001  docs/prds/self-drive-wake/001-throttle-ledger-cold-start.md
a8088c20b024498227382917fc9c95d089f2074a631d793d0167a0cd7006eeeb  docs/prds  prd/self-drive-wake#amendment-002  docs/prds/self-drive-wake/002-throttle-host-umbrella.md
598ab299c7bce2bd02985da186aae771a65ca4de9daa231a0261eb217f6cd200  docs/prds  prd/self-drive-wake#amendment-003  docs/prds/self-drive-wake/003-downgrade-drain.md
30dbe2b520ec27607141a055d74547b524437552f460f9402a461c03379913ec  docs/prds  prd/self-drive-wake#amendment-004  docs/prds/self-drive-wake/004-review-obligation-recovery.md
2e57c57b208715842a1eb6333d94655604f71086d71ce00112dc9c7cd9331e44  docs/prds  prd/skill-command-prefix#base  docs/prds/skill-command-prefix/000-base.md
7a705eaf9fee5ff960fe43dde59c9ef7e853cea9b24cbe53304f91aec717712a  docs/prds  prd/tech-design-brainstorm#base  docs/prds/tech-design-brainstorm/000-base.md
02632a2a8c0f03a89c2db3d8c94ad60af654cb75c94f37ce642c299ff1f72c5f  docs/prds  prd/tech-design-brainstorm#amendment-001  docs/prds/tech-design-brainstorm/001-question-legibility.md
adee2a2ff921928426d5ff333d8ed7bd4437288e4e31d48a5dc7b3276a3561f2  docs/prds  prd/tech-design-review#base  docs/prds/tech-design-review/000-base.md
dcdcb44387b8dabdad1d4e6bce7b7e075a02b6f2eca3623271cf39353c94efa9  docs/prds  corpus/vnext-requirements#requirements  docs/prds/vnext-requirements.md
f99bcd040b6cb11daadbbc66024a1ba5417d50ebe280c422f5f2781207bce3dd  docs/prds  corpus/vnext-self-improvement#requirements  docs/prds/vnext-self-improvement.md
6202fcc0345ed2dc3d07a6ce2bbe2b6e89889c50b7a4fd9a2dd2c284fda2cc80  docs/prds  prd/workbench-ui#base  docs/prds/workbench-ui/000-base.md
917d13b3c01172ccc154719895c5ef7a05e6b991548dd480ff13d6a2e2e50568  docs/prds  prd/worker-transcript-metrics#base  docs/prds/worker-transcript-metrics/000-base.md
f39f834759dcbe82330c6dc99c28bf0d5afc50db581904465f36b48cee556de4  docs/prds  prd/worker-transcript-metrics#amendment-001  docs/prds/worker-transcript-metrics/001-usage-ownership-boundary.md
5351d1d9443b0b07eaec5c67980a6e914137acaa46e107aa6c8641804e3876c3  docs/problems  problem/argus-attribution-hardening#delivery  docs/problems/argus-attribution-hardening/delivery.md
1db3040789f2c7060e09f320a86164f9455293df4b523c862ae7947a9b7c4ac4  docs/problems  problem/argus-attribution-hardening#problem  docs/problems/argus-attribution-hardening/problem.md
629bdf57323f13d874c9b33ca2e26fc434076dd23f1bf830ff0fcde905626ed2  docs/problems  problem/argus-boundary-enforcement#delivery  docs/problems/argus-boundary-enforcement/delivery.md
f30ddfc79b545190f3feab115e54c795b7c9e772498e1de1bf45fb4c67576ba4  docs/problems  problem/argus-boundary-enforcement#problem  docs/problems/argus-boundary-enforcement/problem.md
9bafe66f16d0df85cdf6b7a0f056d0268a79506e2da76c31606707c9041e1441  docs/problems  problem/argus-ledger-lint#delivery  docs/problems/argus-ledger-lint/delivery.md
f0a20928ed9760365997b82301eef2790e539e1c6517fb5a8b7881cfd609693c  docs/problems  problem/argus-ledger-lint#problem  docs/problems/argus-ledger-lint/problem.md
fc4c4521e50c76744a0020a3a23c262c583e3c86d1ff37f637b918a3fa94b4bd  docs/problems  problem/argus-unattended-drive#delivery  docs/problems/argus-unattended-drive/delivery.md
10db12dd67f9c9f449dcd69566c3baddb3d6c61074ff872e8d9e8b342ddff722  docs/problems  problem/argus-unattended-drive#problem  docs/problems/argus-unattended-drive/problem.md
5e871e55b8a5ed78b3e3c7725a997801022e65a3d7f94e0023def39d31850cc1  docs/problems  problem/maintenance-sweep#delivery  docs/problems/maintenance-sweep/delivery.md
de633079d5f6a28e99e7739339b1c6c49201b234412fcc538a164954e01f15b9  docs/problems  problem/maintenance-sweep#problem  docs/problems/maintenance-sweep/problem.md
bf0b6e2331412fc2d1c0c75f8a99a45250833e85bf540d40819438710e487808  docs/root  governance/product-vision#vision  docs/product-vision.md
ea5a9674bcc0cc93fe501d3b5bad0bed9881cc0d2add1ab1fa7d242885b6e773  docs/research  research/ai-software-engineer-research-study#study  docs/research/ai-software-engineer-research-study.md
5225c53c2a8f63a248d72022ba09acab3eb074d9678378b7834405f57f32c98f  docs/research  research/harbormaster-scheduled-watcher-lessons#study  docs/research/harbormaster-scheduled-watcher-lessons.md
637abb18dd96623f375d89499e47e5fc84e79a0cb9e4f8104968c9ed8501f357  docs/research  research/harbormaster-self-drive-failure-taxonomy#study  docs/research/harbormaster-self-drive-failure-taxonomy.md
92d189d94902f0025c96de493fecce34426cb83be15a39c9aae1520178993377  docs/reviews  review/acquisition-trust-v1#report  docs/reviews/acquisition-trust-v1.md
8119103e87386d987722afd44d9963b3be3ece10fa9133a9498d40a644ec8c65  docs/reviews  review/argus-000#report  docs/reviews/argus-000.md
895779207ae935163a384ad359edbee06dcd542a7a24b6aa7801bf828916e7c4  docs/reviews  review/argus-agent-v1#report  docs/reviews/argus-agent-v1.md
0401ac901b222aaf6b080d1dee2154e8fefa34a34ae1a0c73d6711db3e4ad3a2  docs/reviews  review/argus-artifacts#report  docs/reviews/argus-artifacts.md
80a15d2aceadf20b3154e6c99b405635a05516ef19b91d6f2abfe2597e41c565  docs/reviews  review/argus-attribution-gate#report  docs/reviews/argus-attribution-gate.md
5de4fe462dc73f6afac25204a60e597f1570edffbe9e4340daf29560a7f4db55  docs/reviews  review/argus-attribution-hardening#report  docs/reviews/argus-attribution-hardening.md
38eb663d613cfec0e856827b1e4f5f237d305e0ae75f9645f3874b59f965ffe4  docs/reviews  review/argus-authority-contract-structure-arm#report  docs/reviews/argus-authority-contract-structure-arm.md
06eec8e0047fa301bb0a811bb9f37c3d91c3325da2a7d42209dba8c1f2835b6e  docs/reviews  review/argus-boundary-enforcement-000#report  docs/reviews/argus-boundary-enforcement-000.md
71729897ed588f329f687a43de9f51bdebdcd840482fb931d070f264edd2b8a2  docs/reviews  review/argus-boundary-enforcement-001#report  docs/reviews/argus-boundary-enforcement-001.md
caeb15cb0d16521985bffa33608d9eee2fb8b5f4fdfc956bfa94105679245faf  docs/reviews  review/argus-ledger-lint-000#report  docs/reviews/argus-ledger-lint-000.md
28af2cd6a80f800dee06a7ca0de8dc4d4f6e34ca8e270fe668e4f745540eddac  docs/reviews  review/argus-ledger-lint#report  docs/reviews/argus-ledger-lint.md
22e946521e40782bed01a906b0da41babf51cb1734e031e85aa8614f6740532b  docs/reviews  review/argus-unattended-drive-000#report  docs/reviews/argus-unattended-drive-000.md
e66eb661496d11b280f173e116cdea3b99615d6918d65d38d5eb68f32319383f  docs/reviews  review/argus-write-hook#report  docs/reviews/argus-write-hook.md
c4e6b2c96fff09879e74f8d268c8a4a19db7be8b4b2e561cc50186b22c0d1274  docs/reviews  review/channel-inbox-000#report  docs/reviews/channel-inbox-000.md
3ac56210fa3cb50b89784f5a2fe9bb11dd3164bddf962f0d250cd842e4850db5  docs/reviews  review/code-gate-mirror#report  docs/reviews/code-gate-mirror.md
0e8d783e8e67916b086d859bde936a1700687089945949a95dfae16d14d28d40  docs/reviews  review/codegate-v1#report  docs/reviews/codegate-v1.md
21206b18090574adb462c2388256872a71ee27549c58b39768a0f7a9aa67d5fa  docs/reviews  review/config-driven-portability-000#report  docs/reviews/config-driven-portability-000.md
a201087b54c07e4995c8d43fc0c0987195e40bfd7a001c0381d9c1fb299e165d  docs/reviews  review/config-driven-portability-v1#report  docs/reviews/config-driven-portability-v1.md
ef6f80a75479227ee2b6c348aea26a4c3ab0b36e71083b5bc8d76c4b1b52cd60  docs/reviews  review/contract-gate-000#report  docs/reviews/contract-gate-000.md
217f2db449d0a1822999862aa899fefc4a51896691f044c516c3f502ea75acb3  docs/reviews  review/contract-gate-001#report  docs/reviews/contract-gate-001.md
18305c34fe6028c15537257be21d77e0a1407dac65ffb38c46ee0838cebcc1d1  docs/reviews  review/contract-gate-v1#report  docs/reviews/contract-gate-v1.md
eb4699e030167fac26c43ef54900b8648b1dd7deb21d5ae1548b74ec53af4139  docs/reviews  review/docgate-018#report  docs/reviews/docgate-018.md
dc0310891b9236bf2d6d1a26feb146472ff8ed2590a4188d146c9e63c7c158a3  docs/reviews  review/e2e-verification-000#report  docs/reviews/e2e-verification-000.md
e9b81ed6e0f69afdbcc34acee9771ddc28341135736cc3475f22035b5d2cb2c8  docs/reviews  review/e2e-verification-002#report  docs/reviews/e2e-verification-002.md
d67e834462cda90527ed576b07782226ebb8c733b3f41cfaf83d358cf0401e49  docs/reviews  review/e2e-verification-artifacts-v2#report  docs/reviews/e2e-verification-artifacts-v2.md
84230ab969544becb7ec138b90eceece43645e23c364ee49e5d253fd010ccd63  docs/reviews  review/e2e-verification-artifacts#report  docs/reviews/e2e-verification-artifacts.md
643a0fc662d40c1dc4bfc0f8d236451b0a204b2bf6e757be1322deecaa7f48db  docs/reviews  review/e2e-verification-skills-v2#report  docs/reviews/e2e-verification-skills-v2.md
72b035d55ea40de44144a6b31ead333b4d191ea01ba46008af33c6204f8a2d69  docs/reviews  review/e2e-verification-skills#report  docs/reviews/e2e-verification-skills.md
71964070658d4a322420b3db2656c81e31db406d04a62face2fe056c34d60c8e  docs/reviews  review/ecosystem-help-000#report  docs/reviews/ecosystem-help-000.md
8f734e4fe4b65804b531d9481da92e7863f0e66dee5f7410d724894d9607d639  docs/reviews  review/ecosystem-help-v1#report  docs/reviews/ecosystem-help-v1.md
4e0c1fe55f2702cffdb813b19b9bf432fa61e8f09dc207132a1c21efcacee747  docs/reviews  review/execution-demand-contract-v1#report  docs/reviews/execution-demand-contract-v1.md
bfe478f995ef3bd46c2587f674deba8cb93debedcc58dbc28f7259d2736bd5cf  docs/reviews  review/existing-repo-onboarding-000#report  docs/reviews/existing-repo-onboarding-000.md
04a682026e2d25f477305f3b7ee2468b01ad4337228cbb235214b2798abf05a8  docs/reviews  review/existing-repo-onboarding-v2#report  docs/reviews/existing-repo-onboarding-v2.md
4d5cc82272bc90e19e0e1357bf7dd8f1ab37f48fd17d455a3cd1fd32568b6459  docs/reviews  review/feature-brainstorm-002#report  docs/reviews/feature-brainstorm-002.md
f02531020c8a18db58b6bb0081bd235e0190b91eddb1fde67e7cc098e75dc036  docs/reviews  review/feature-brainstorm-003#report  docs/reviews/feature-brainstorm-003.md
6554b32da320b14fb7afa47e0017d6b94bb6ed1ab7a7f5a86b0f1c1996b50175  docs/reviews  review/feature-implementation-drive-000#report  docs/reviews/feature-implementation-drive-000.md
aa92bfd4ca825ac57d3892d130a1d86a52d36b0ac51b04d8dba823ac331ea031  docs/reviews  review/feature-implementation-drive-failure-contract#report  docs/reviews/feature-implementation-drive-failure-contract.md
61459b7af7f5a6f7dfe1edef8c88cf9f2bef6ba627187409b262c4be2780fe95  docs/reviews  review/feature-implementation-drive#report  docs/reviews/feature-implementation-drive.md
d87a6c28ba81956fc3bb069cec89aab637e4951205fd8ac15186703332494f42  docs/reviews  review/fleet-bridge-v1#report  docs/reviews/fleet-bridge-v1.md
1f69319a5b28112097dd5ac5ed38402fc382fdffbc431efba5aacdc4fc96661f  docs/reviews  review/impl-review-000#report  docs/reviews/impl-review-000.md
fb4af40a51c8fdc9fb573e6a0fa4ef2da71035d85bc6f67b957e3ad1876ff894  docs/reviews  review/impl-review-002#report  docs/reviews/impl-review-002.md
0538ac345110786bde6e13cf58b2d351253c8efedd7acecfb4876d9bf09e0828  docs/reviews  review/impl-review-003#report  docs/reviews/impl-review-003.md
7171e71350af0b93795e3087b0623b4277fa636f3a302637280e24b62633137f  docs/reviews  review/impl-review-004#report  docs/reviews/impl-review-004.md
014c541fb48823669d254cd24de4c9d4283a6e0e3a4e7864b73d6a5eb24f1e28  docs/reviews  review/impl-review-host-persistence#report  docs/reviews/impl-review-host-persistence.md
c409f5e9bce0122da9239f6f05e2fa83eec5e346dd77b723214ccc560bb272a6  docs/reviews  review/impl-review-routed-assurance#report  docs/reviews/impl-review-routed-assurance.md
5ef38c800f68911dab6c45e1d42a37a070198aa107cce53d880f0cc1cc092e58  docs/reviews  review/impl-review-v1#report  docs/reviews/impl-review-v1.md
2eccc10f1e12236f33897b0187e869965e0948f3ff95bb0ebc5f216e1373f668  docs/reviews  review/implement-now-001#report  docs/reviews/implement-now-001.md
52f22eb154529d05fad80af4225b3e59cd7f29b8f3ede1a17b1a20eadb9bbc95  docs/reviews  review/implement-now-002#report  docs/reviews/implement-now-002.md
7792917831fc74a3299185db8ba99ecbc4579248ae0059f14ba7fde0f2d52740  docs/reviews  review/implement-now-003#report  docs/reviews/implement-now-003.md
52bf93bdb30c52017aa126040b036c21c38e30383f5b198368e33ab74b15ee21  docs/reviews  review/implement-now-004#report  docs/reviews/implement-now-004.md
c230b8be85f41aa2d5e5ceb8a54e8c45618ebb57ccce3ca9b723496075acdee3  docs/reviews  review/implement-now-005#report  docs/reviews/implement-now-005.md
5a69bcd84c6485d13c17df553dec1cae68ce1641e3bfb3b0510838ed8b53a289  docs/reviews  review/implement-now-006#report  docs/reviews/implement-now-006.md
620e07ce788b00ca4f8f4701667fe5e0f58d52d62bd888200d7b74f764237692  docs/reviews  review/implement-now-007#report  docs/reviews/implement-now-007.md
a5f875adc6d8a30d6f635d55eed17de4982e4e22ca374dc10aaf783caa60435a  docs/reviews  review/implement-now-authenticated-review-obligations#report  docs/reviews/implement-now-authenticated-review-obligations.md
a2407054bd438f55136361dc63a27c6945b6f0b0671136b471d629beec4f8b6a  docs/reviews  review/implement-now-e2e-tasks#report  docs/reviews/implement-now-e2e-tasks.md
3bae238a1903d0f8d3dde7aeb828d48e99ddb64011312c1b5497d8c1f221d065  docs/reviews  review/implement-now-routed-execution#report  docs/reviews/implement-now-routed-execution.md
c282a4c529111f7112a25a0cff12e3bbf965c6a02672a2a1513a8d6b2b5f628e  docs/reviews  review/implementation-verification-000#report  docs/reviews/implementation-verification-000.md
f1fcb597c4ffe27d47dbc3a774525830b65979de081c02df3e77a865a87e574b  docs/reviews  review/implementation-verification-001#report  docs/reviews/implementation-verification-001.md
24ba3791358c21d18ba9b52c28d8fe7c761214cda4c07f308d0abe66255e64f9  docs/reviews  review/init-hardening-000#report  docs/reviews/init-hardening-000.md
c32bc61c65463b66a9800001e012d8ae4af24c95dcf15d8fb77ea0efd5720fea  docs/reviews  review/init-hardening-001#report  docs/reviews/init-hardening-001.md
530cc8c8508389e70419542fce066617d2968467c50fe5e918e75231bceae2fb  docs/reviews  review/init-hardening-v1#report  docs/reviews/init-hardening-v1.md
3cafc72151c4175c9013e39f2767d5fb0187f628fa9a73312424df0acd7492a1  docs/reviews  review/installer-000#report  docs/reviews/installer-000.md
aca4f3bd3cbb860e306a8a516e1b7cd22127a4837792ff5aba099657ac8129fb  docs/reviews  review/installer-001#report  docs/reviews/installer-001.md
d868a80493f7cfd17289983422ba64ee923f4c621151cb98bb22ad2b52c05fab  docs/reviews  review/installer-003#report  docs/reviews/installer-003.md
52fd7143c5214add5179ad9fa7aa5a4495f7bf570802dd69aef267c1f89cafbb  docs/reviews  review/installer-v1#report  docs/reviews/installer-v1.md
f5b98eed4470242312438e7315432f931fbcc017fe187612403281488a703521  docs/reviews  review/local-work-tracker-v2#report  docs/reviews/local-work-tracker-v2.md
154a78af0e940c5233e4d23c2d7353213f855299dcd90ec61ca5ce3db580b774  docs/reviews  review/maintenance-sweep-000#report  docs/reviews/maintenance-sweep-000.md
39942e9505d25bfea29214cd73a4ec6ec3d3749354778cb86bb9674c97e2846a  docs/reviews  review/maintenance-sweep-001#report  docs/reviews/maintenance-sweep-001.md
a8adc6153a58c9c291e4841102ab40c09c6657a59f5c9217a81d51cb4e2bec8f  docs/reviews  review/maintenance-sweep-v1#report  docs/reviews/maintenance-sweep-v1.md
65d597d31fb7fa98fb7f563b4fa0daef8881f785b9f25f7def18f8ba9f1526ac  docs/reviews  review/merge-lane-v1#report  docs/reviews/merge-lane-v1.md
53dc258d1ffd7f259b583b9d27fa1c4d7704b21ffdf09004f37cb1ce15c228a0  docs/reviews  review/native-routing-policy-v1#report  docs/reviews/native-routing-policy-v1.md
8d5e74bf1e19e6ad53606b301eb4f80a6b1ea4074b61909b5c2fe71fc2715a37  docs/reviews  review/pipeline-measurement-000#report  docs/reviews/pipeline-measurement-000.md
c624f639a699fdc0e577b68642223cc8a46c035c554f54a0cd0432c1300b0b1d  docs/reviews  review/pipeline-measurement-v2#report  docs/reviews/pipeline-measurement-v2.md
fa92f87f5d59509e8fc800a4cd12ffdf7d8c4dc332d2d33c1017eac2c66e50fd  docs/reviews  review/pipeline-measurement#report  docs/reviews/pipeline-measurement.md
e318d9622c58ef71ed47ec159947f4872432d568eac282c4135b7aba37ccce76  docs/reviews  review/plan-new-004#report  docs/reviews/plan-new-004.md
cc0bd998b31db5b11e49225cdec3e8b8578f935b7c6a27de63119830444988e0  docs/reviews  review/plan-new-routed-demand#report  docs/reviews/plan-new-routed-demand.md
77a4eb61ad527f33d0837ea652263b37f8d774cc78a9462fd5683d2de9399d8a  docs/reviews  review/planner-driven-model-routing-000#report  docs/reviews/planner-driven-model-routing-000.md
6ca84fbb7ac2c0ac3d4f585524086d361eec1d6ae70002d8a4ae6208f2f7cecf  docs/reviews  review/planner-driven-model-routing-001#report  docs/reviews/planner-driven-model-routing-001.md
bbcda498f155bed7ae2d771bc8cb0bf3c0c3d486a83d77290a2482951b0505ea  docs/reviews  review/planner-driven-model-routing-002#report  docs/reviews/planner-driven-model-routing-002.md
d4a79f238c6d9af9fed7514ace160f36c1976135eb184e29e61435ba553969ca  docs/reviews  review/planner-driven-model-routing-003#report  docs/reviews/planner-driven-model-routing-003.md
45851a627a73210878d5a25e4edd3e7dd58407b5327c4ef9afb11966267fb189  docs/reviews  review/prd-amendment-model-v2#report  docs/reviews/prd-amendment-model-v2.md
4dab2d25590113400553d3533c186341c0ce3a244c6714c120fc3e2386fbfb1a  docs/reviews  review/prd-review-001#report  docs/reviews/prd-review-001.md
bbe59c4805a3daef20c83ecee1f36e8a055c3f9dcca17023ec466a22f57d605b  docs/reviews  review/read-to-rule-000#report  docs/reviews/read-to-rule-000.md
83d6aab85cd479236031e6befb75d302fddcc134150ba7fb4401756808939313  docs/reviews  review/read-to-rule-tech-design-v1#report  docs/reviews/read-to-rule-tech-design-v1.md
83aa5c66a16a3ba814c56fe5720f406fd1317e1bf074b2934a89cb55d0780f5d  docs/reviews  review/read-to-rule-v1#report  docs/reviews/read-to-rule-v1.md
9cd8e4f5a93f52b9a4a20a4b2a9c5a4530d1e6bfa413d5e232c4792ba5c25116  docs/reviews  review/review-feedback-capture-000#report  docs/reviews/review-feedback-capture-000.md
c65e2b9a810a11508e2118ed41659beb563701b6f238dda062b4bcf898065ce5  docs/reviews  review/review-feedback-capture-001#report  docs/reviews/review-feedback-capture-001.md
963a5ffeebb8bc9760f9f8fd8f4144b52dcdcb4c6a7a927f2c29c79a7e6e5575  docs/reviews  review/review-feedback-capture#report  docs/reviews/review-feedback-capture.md
b33535efa448cd0d8322640b9dc79054820d70b5b2f204c24fad775d9cc2e2f0  docs/reviews  review/routing-evidence-v1#report  docs/reviews/routing-evidence-v1.md
a9637b3ff42b9a30ba03a59f54ecbf943467f69b9a2286618d9612626b2a1c3f  docs/reviews  review/self-drive-wake-000#report  docs/reviews/self-drive-wake-000.md
bc4f178fe548d8a906f0a5d68fa49b953ba99b603c43f0c0a05b8c6598705f98  docs/reviews  review/self-drive-wake-004#report  docs/reviews/self-drive-wake-004.md
75013388680241c3398d660dbe2c16d6e2b939b5809e5f0832f406f4ac91ba52  docs/reviews  review/skill-command-prefix-000#report  docs/reviews/skill-command-prefix-000.md
277efd0d67ad6c40a5c94578b3737e90a40df6d24dbae70aa66deee19837e207  docs/reviews  review/skill-command-prefix-v1#report  docs/reviews/skill-command-prefix-v1.md
5991af972a40edf093a4dee375cc77443bb0387ee7bbf8f57670801ba1e66298  docs/reviews  review/state-reader-v1#report  docs/reviews/state-reader-v1.md
6ce5edca0cc7743b45991a0b875bb5a2ca2b82e5cde9f1d3123654f6e743f713  docs/reviews  review/state-reader-v2#report  docs/reviews/state-reader-v2.md
6d3b7a68ce747d34c22f92148ede3df635acbb7d470ccf7d57a11feac800f5e7  docs/reviews  review/state-reader-v3#report  docs/reviews/state-reader-v3.md
47e2aea6dad79e4fa434e099713a79acd8203868564462a0ced78adf1f428803  docs/reviews  review/state-reader-v4#report  docs/reviews/state-reader-v4.md
ec41d635eba34d754ed3c8c8e6442df4fbc4e8cd601ed501810e5245047d16c1  docs/reviews  review/state-reader-v5#report  docs/reviews/state-reader-v5.md
78fb774b2fe40fb1ae51a557582b79c2e6ec136e5c6a4f5860cd316739b094a3  docs/reviews  review/tech-design-brainstorm-001#report  docs/reviews/tech-design-brainstorm-001.md
88dbf4a6b85d7305ead97425dcfefeafac0463e8abbc8457026b7c4fa30dd86e  docs/reviews  review/wake-channel-v1#report  docs/reviews/wake-channel-v1.md
67901c261c048eb601d75cf18cce89ff73db9d7d998785e4512d8aa17395d82e  docs/reviews  review/wake-watch-v1#report  docs/reviews/wake-watch-v1.md
fdf3496bad144b517010032958c3d53856a7b50bfc3ea9ae8dc3d41295dcaef8  docs/reviews  review/wake-watch-v2#report  docs/reviews/wake-watch-v2.md
029a20ab110fd69bbb814a55e4aee630ad4b3c62b94aca3beeae1f2cdf238442  docs/reviews  review/wake-watch-v3#report  docs/reviews/wake-watch-v3.md
28664aef9be61fe094f5a956eee2359c327e873e932a74305e9e9429f1c157ce  docs/reviews  review/workbench-ui-000#report  docs/reviews/workbench-ui-000.md
2070abb31a29aacdb103ed511192217348e0400d75eb2426985781e8ea752643  docs/reviews  review/workbench-ui-v1#report  docs/reviews/workbench-ui-v1.md
b4ca8b22eae2159d4f3023e1bcbef29d58a021fd597eee4a2b76e7bf6ec4486c  docs/reviews  review/worker-transcript-metrics-000#report  docs/reviews/worker-transcript-metrics-000.md
19dad1a283ecc10f86c61b261bfe31b62bd398889dda447716ecbe7dd2a51042  docs/reviews  review/worker-transcript-metrics-001#report  docs/reviews/worker-transcript-metrics-001.md
472c5fb7e2c82b5e6b69ede711ddd6940efb70c251f471c5c306513298c051b6  docs/runbooks  runbook/merge-lane-activation#runbook  docs/runbooks/merge-lane-activation.md
90d2ed2e697baa170c59a533e32780124604d74b1d9fc13339ee4e478d0f4b1c  docs/runbooks  runbook/merge-lane-workflow#workflow  docs/runbooks/merge-lane.workflow.yml
86bd90815776062b1963b15a994a8f410c2c4650609667fe33018b5f9df77f95  docs/solution-shapes  solution/installer-v1#shape  docs/solution-shapes/installer-v1.md
b300657436911a285ad4110a89c24d78e089f4d64fea1384d0059cc544d159a0  docs/solution-shapes  solution/maintenance-sweep-v1#shape  docs/solution-shapes/maintenance-sweep-v1.md
ca32d1b77aaa34fbcc205766ec8f571e1b4285dd56a990f1404c1c193e0e6bfa  docs/solution-shapes  solution/merge-lane-v1#shape  docs/solution-shapes/merge-lane-v1.md
331bbffd030177d2a7221c97c1edf1c0baa5b81544c0918775d6e196c9911858  docs/solution-shapes  solution/plangate-carina-adapter#shape  docs/solution-shapes/plangate-carina-adapter.md
a0b3d0e69f762ca7af101a5af76d249d8c956d27f39db0f6709545b53fe59ac6  docs/solution-shapes  solution/plangate-slice-coverage#shape  docs/solution-shapes/plangate-slice-coverage.md
b9f2980086feae96071811a12dc4d2e28cfd3f6edc08230f165e2f6970848725  docs/solution-shapes  solution/workbench-ui-v1#shape  docs/solution-shapes/workbench-ui-v1.md
1b42d98261bcb79c52034f5d997769730b55b7c2162f0cabef57c104202ca1b1  docs/solution-shapes  solution/worktracker-hierarchy-index#shape  docs/solution-shapes/worktracker-hierarchy-index.md
08058a7b2c7d9fe316a9fdd1923eea8618d620657b7200dfe359c9db1ebb8330  docs/tech-design  design/acquisition-trust-v1#design  docs/tech-design/acquisition-trust-v1.md
8819235c5a03ebc20e8a41a309d7b00947d68c83161ce5dcc77d96f870647ba0  docs/tech-design  design/argus-agent-v1#design  docs/tech-design/argus-agent-v1.md
8dc33c9a39f15198ce210bc812c53d02af3d5705440dc4da5b52660419b4df20  docs/tech-design  design/argus-artifacts#design  docs/tech-design/argus-artifacts.md
0b0c953fbe8665bf9ddf9fd184d05ee312aa461c0cb2151ea2aaf8b22400cefc  docs/tech-design  design/argus-attribution-gate#design  docs/tech-design/argus-attribution-gate.md
6b7254e792e9691ce7a93074a9043414715aeec69294eccdcd1d1cde74efda7c  docs/tech-design  design/argus-attribution-hardening#design  docs/tech-design/argus-attribution-hardening.md
1bb594d884b7ae8c1090022eceae77745aee55b6e8cfb17bd7393a1b159d4163  docs/tech-design  design/argus-authority-contract-structure-arm#design  docs/tech-design/argus-authority-contract-structure-arm.md
b891b13b932a14dcff38502e08c52cc2f79ab2f9aad8edaa9fab17d516f8fb9a  docs/tech-design  design/argus-ledger-lint#design  docs/tech-design/argus-ledger-lint.md
dfc6f23a4cc2dece296b198784ba922223f6de3cf3c1852c94d19d33d77b9cde  docs/tech-design  design/argus-write-hook#design  docs/tech-design/argus-write-hook.md
324ace5fc103364f8435ab4e7e70819b10a37959d2c654309341b0ebd61a6995  docs/tech-design  design/brainstorm-record-model#design  docs/tech-design/brainstorm-record-model.md
cac96f4349b575c0b7e5483f1b23687d97b9d1d441813ff5abdb486509ae55bd  docs/tech-design  design/citation-resolution#design  docs/tech-design/citation-resolution.md
4829ded8af479a35f457c7034c85f4b6b5da0be758f8c22edfd3d5d0e95b0674  docs/tech-design  design/code-gate-mirror#design  docs/tech-design/code-gate-mirror.md
fb47372869e19517afdc80e696a1c7a2f8f1c8c38cc5b0f04abda68b28263410  docs/tech-design  design/codegate-v1#design  docs/tech-design/codegate-v1.md
8d25ba0f65eb1bbed82e3f85efa63eb9791ff1246eb0fc44fa32f665307dd273  docs/tech-design  design/config-driven-portability-v1#design  docs/tech-design/config-driven-portability-v1.md
0eda89b54f774ceeba16b812a431c85f38941582676f642e93662ed26fe49f3f  docs/tech-design  design/context-pack-v1#design  docs/tech-design/context-pack-v1.md
b73966f7ed110eeffdf3640c0243477cc0c78b96ed2c9bb5a4a24f4909e5cbe8  docs/tech-design  design/contract-gate-v1#design  docs/tech-design/contract-gate-v1.md
3c9d24b2a83af985594ff74ae5a2e06fd6b0790ba7b3b710b130fd25c07f15f3  docs/tech-design  design/coverage-on-read#design  docs/tech-design/coverage-on-read.md
7bfc819d446e88b0f258bad6b410e95e2a2c0556fb68a2ea08df6f579a714aae  docs/tech-design  design/design-brainstorm-record-model#design  docs/tech-design/design-brainstorm-record-model.md
6b20e2c7691634af8abe8fe944c23ca2bf4a6a3af0158ee8674ee882b8e940e9  docs/tech-design  design/distributed-check-registration#design  docs/tech-design/distributed-check-registration.md
d8c207fec2497743c6399550310e0534361e3ca73b181f04a8bd34780064520d  docs/tech-design  design/doc-link-integrity#design  docs/tech-design/doc-link-integrity.md
3d8e1c1151e97cfb8f965613fe1f5596ac992406091413f37868999206e4d55d  docs/tech-design  design/docgate-fence-completion#design  docs/tech-design/docgate-fence-completion.md
886df1bfe3cf46f088dc222eb71cd00eff715d3a55f530c4bbbdf79fe022af88  docs/tech-design  design/docgate-v1#design  docs/tech-design/docgate-v1.md
9dbf41b6b24e568cd940650897b903703d4caef34d3a725fb0a14bb7d8d3a6a2  docs/tech-design  design/docgate-v2#design  docs/tech-design/docgate-v2.md
779a9cab36d326f16934019e5c4c24f510597f439dc70edd35e6df1b3dc9409c  docs/tech-design  design/e2e-verification-artifacts-v2#design  docs/tech-design/e2e-verification-artifacts-v2.md
655e7a5a29613004519944275a1e04150fab3c183782317d97e492bc8109cff6  docs/tech-design  design/e2e-verification-artifacts#design  docs/tech-design/e2e-verification-artifacts.md
c0e92b035008945c7a680f55db892690be2f5f4a1e831350bb069e76a776ed35  docs/tech-design  design/e2e-verification-skills-v2#design  docs/tech-design/e2e-verification-skills-v2.md
30f57e57d308d5e3be2e699fec1ce576a59d1e4a75af4bedb2f786809462222d  docs/tech-design  design/e2e-verification-skills#design  docs/tech-design/e2e-verification-skills.md
0fc642c5472e391f37e5f82e2487c138cd820ba48209ad38b8708605263acfe2  docs/tech-design  design/ecosystem-help-v1#design  docs/tech-design/ecosystem-help-v1.md
1803a578f24d1fcd4e72fe323beac1ad9b7a2b7372c40c5c9193c843565778fd  docs/tech-design  design/execution-demand-contract-v1#design  docs/tech-design/execution-demand-contract-v1.md
7543d903ced08ca495c414591ef518abe45b1297bae41fa226a31c1421e00fbb  docs/tech-design  design/existing-repo-onboarding-v1#design  docs/tech-design/existing-repo-onboarding-v1.md
c7cc38bb76acd3c8c1fd815af93bd17c9784b6970dc137db8217b9211f7dea5b  docs/tech-design  design/existing-repo-onboarding-v2#design  docs/tech-design/existing-repo-onboarding-v2.md
4bc6c0fe437a7b8d230535dce4519b0c6a06254682764694633b95361f07d599  docs/tech-design  design/feature-implementation-drive-failure-contract#design  docs/tech-design/feature-implementation-drive-failure-contract.md
9ea7096e912167ecb729a6e4e1b927caa05db54140c93a34e0549e41ee37d4cf  docs/tech-design  design/feature-implementation-drive#design  docs/tech-design/feature-implementation-drive.md
5caf70f8c045d82995e2034d8f564b430b9b83ff89f853a639087478382c9f15  docs/tech-design  design/fleet-bridge-v1#design  docs/tech-design/fleet-bridge-v1.md
4baa323ea0fe7b75446dbd91387e5b95461a791f448fad4ef6bb4ff5751d8099  docs/tech-design  design/impl-review-host-persistence#design  docs/tech-design/impl-review-host-persistence.md
57c09708c962287c08b4eed955fbd6eb4259039f76828f1b9fd9f155044d517c  docs/tech-design  design/impl-review-routed-assurance#design  docs/tech-design/impl-review-routed-assurance.md
9a5dac7be3faea7f34583c87e8ee051c78afa92cdfb0850966fb2b3c0531464a  docs/tech-design  design/impl-review-v1#design  docs/tech-design/impl-review-v1.md
9fa45230795fa50115f9fd080951789e2b8f4f40e60c600f7484e0dde70286c0  docs/tech-design  design/implement-now-authenticated-review-obligations#design  docs/tech-design/implement-now-authenticated-review-obligations.md
43762eb5722d8ef8f813bd0ee78c8066d56074c2e41e3230bdbea7902a7076b8  docs/tech-design  design/implement-now-e2e-tasks#design  docs/tech-design/implement-now-e2e-tasks.md
ef75ac9e1c5a795e11b8a71cfadce65ac02dd6605e60970267b56e4dd19556b1  docs/tech-design  design/implement-now-routed-execution#design  docs/tech-design/implement-now-routed-execution.md
bccb4f6c180cb1ee20254f22ac70bdabf63daa252f3ebd7aeaaa7aaa3491b4e8  docs/tech-design  design/implement-now#design  docs/tech-design/implement-now.md
4278031c04df70f1e41423635d309ec7030a7703f3b056cf104f05a4e6a3bf8b  docs/tech-design  design/incident-machinery#design  docs/tech-design/incident-machinery.md
bad1558f61da9be889d34a67b74fbb77e7a05482ead2347905d498b6a30231c8  docs/tech-design  design/index-coverage#design  docs/tech-design/index-coverage.md
f1710649feb9b92664f344798b9e17d86efdcab1acaf8ee7bba343363531750c  docs/tech-design  design/init-hardening-v1#design  docs/tech-design/init-hardening-v1.md
5c77436e03f67de06fe736131bde819d3bf27ff6f476a4d7e44ccf41eaae1618  docs/tech-design  design/installer-v1#design  docs/tech-design/installer-v1.md
10d1fc8c567108e38fb3e8dc670705e1a1a8a1f7cfebf1f20242f4c03067a95f  docs/tech-design  design/local-work-tracker-v2#design  docs/tech-design/local-work-tracker-v2.md
f99c68d58137140a9e338456a2a018f07e07e62a61a6727bfb7d2a30d526b54b  docs/tech-design  design/local-work-tracker#design  docs/tech-design/local-work-tracker.md
b49c1108056cb6e27cfdc8f53a55107ec21c42fdc8b75770b06d9bdfc579d2e8  docs/tech-design  design/maintenance-sweep-v1#design  docs/tech-design/maintenance-sweep-v1.md
c32e493cfe971b356c08109d96d2c1b8f2436e7c1a00f6abc087c3db490d37e7  docs/tech-design  design/merge-lane-v1#design  docs/tech-design/merge-lane-v1.md
df3278e35a16498224497dc3c18bffb19ac339ec3c6fa91585f47f302028a062  docs/tech-design  design/native-routing-policy-v1#design  docs/tech-design/native-routing-policy-v1.md
848be6bb1101995e3e698ade196fe74ee1eddf3150285b46b2e557a5764b59d6  docs/tech-design  design/pipeline-measurement-v2#design  docs/tech-design/pipeline-measurement-v2.md
ff2eff2304f4b8c4b70b9259aa4ae5299f0d1c82ba183578549ddabd0059eaf8  docs/tech-design  design/pipeline-measurement#design  docs/tech-design/pipeline-measurement.md
2ab87137b5d7a8635d2a5dc697e7525dd8de80f53ec455b977d7afc3d7033dc2  docs/tech-design  design/plan-new-routed-demand#design  docs/tech-design/plan-new-routed-demand.md
0135008dbc7bb9724b513c9f2f8abd50de84fea984e75fa7b477a44ac17ca966  docs/tech-design  design/plan-new-v1#design  docs/tech-design/plan-new-v1.md
5db445f7902743ef21b283d70e7f9bd9a10be2558f3cd7d76662ea8bb7dede5d  docs/tech-design  design/plan-viewer-v1#design  docs/tech-design/plan-viewer-v1.md
647f94d27667ba074481e31c553cff0b9eb989b1fb0fbd67fb0542abff257beb  docs/tech-design  design/plangate-carina-adapter#design  docs/tech-design/plangate-carina-adapter.md
0ea83e322d0283b789882107f66b09d9c23df47b2e6c185c511a30a16d45bd6b  docs/tech-design  design/plangate-slice-coverage#design  docs/tech-design/plangate-slice-coverage.md
552a8df2de15390d67bbff1fea20073207c7ec97b9198c4e72230f59f77874f6  docs/tech-design  design/prd-amendment-model-v2#design  docs/tech-design/prd-amendment-model-v2.md
7a65d47d00bdf9b5467675536e23a6ab43fb320a836207fb96344be6ca47e9d8  docs/tech-design  design/prd-amendment-model#design  docs/tech-design/prd-amendment-model.md
ff9eada4122511ce035ac08347e3c33ac5f6e3fa860f97dd6f5122b6de1dde1a  docs/tech-design  design/prd-lifecycle-home#design  docs/tech-design/prd-lifecycle-home.md
ebacdece29f692ebe2a5a69e2d06685c6fe55111da91c8173abb3f0770b55185  docs/tech-design  design/prd-review-artifacts#design  docs/tech-design/prd-review-artifacts.md
af1c66d49b3f9588c789f30034e06a4bbbc5c93396f098799e04cea76d63d0f5  docs/tech-design  design/prepush-ref-awareness#design  docs/tech-design/prepush-ref-awareness.md
52e19c121d55934a8bb948e629a4292fb3590b385f28c6d287859214276a4428  docs/tech-design  design/read-to-rule-tech-design-v1#design  docs/tech-design/read-to-rule-tech-design-v1.md
25b1caf111afb7fe5f498975cb5667ba253feffe1c41a59714604108dd2de64f  docs/tech-design  design/read-to-rule-v1#design  docs/tech-design/read-to-rule-v1.md
5c304b45eb1662b114facd8a7f81eede013ac6782e144377ccb052b7bd63e3c6  docs/tech-design  design/repo-layout-and-system-footprint#design  docs/tech-design/repo-layout-and-system-footprint.md
4e3a8105ea45accb67ecd77d65b8eeb2a0f942e1231fc979c04f1bfd1bf82235  docs/tech-design  design/review-feedback-capture#design  docs/tech-design/review-feedback-capture.md
37d678d22dc45a8a0db6556c0597c44025f935c362175aca0a69ff80dfe59a7c  docs/tech-design  design/routing-envelope-v1#design  docs/tech-design/routing-envelope-v1.md
e5aede37de9d27aedcbde713870bd19a84dcbda30559a99ced0819f23f89cf71  docs/tech-design  design/routing-evidence-v1#design  docs/tech-design/routing-evidence-v1.md
f30210e6325058d6db762612d37fb73543593c0f8486ec2d4745ccf2cbdaacc1  docs/tech-design  design/serves-hardening#design  docs/tech-design/serves-hardening.md
3a6b8fea00e859a15e5cefba04a544e9cb943e9e3b66488645d62bb1b1923087  docs/tech-design  design/skill-command-prefix-v1#design  docs/tech-design/skill-command-prefix-v1.md
55276febfbea20a48b3db61ce24d4a473df4869bbb2c1dc52ddc6d9ca7a37c47  docs/tech-design  design/solution-shape-v1#design  docs/tech-design/solution-shape-v1.md
e28fe6b36e48f42694b62ebe679ab2f4ad062a0569de817e6aea2576ef16619d  docs/tech-design  design/standalone-docs#design  docs/tech-design/standalone-docs.md
73f6b4f35e153d72515060c4502a0c01fd3019dbda425661317eb88aaa6fa6bf  docs/tech-design  design/state-reader-v1#design  docs/tech-design/state-reader-v1.md
866026b796b9ea363d9c1b9770938519f12649438feee509e8b88bc125f53ccb  docs/tech-design  design/state-reader-v2#design  docs/tech-design/state-reader-v2.md
8223e4a8fd993362e30596901af18b72766516134d0435c48c04a8e4a00abfc5  docs/tech-design  design/state-reader-v3#design  docs/tech-design/state-reader-v3.md
47d4a97a161bfa79a09ef85ebc4e1e335819097ca5c5d9008788d2a27f7d7cb7  docs/tech-design  design/state-reader-v4#design  docs/tech-design/state-reader-v4.md
48d5fa59cbb34b697742939dffb396ef30122795efa91c4cca7ce1d1f453fa36  docs/tech-design  design/state-reader-v5#design  docs/tech-design/state-reader-v5.md
ffe2502099bb3c7901fdb36203f3dfc35c8dba9d05dcb86e1d9637150091887c  docs/tech-design  design/tech-design-conventions#design  docs/tech-design/tech-design-conventions.md
8a03816c31cc17cbe3becf3fbf15630806982a69b98fbc51693e90bf598d5337  docs/tech-design  design/tech-design-review-artifacts#design  docs/tech-design/tech-design-review-artifacts.md
aae4d57146f02d49d30efa8c027ac2e00f1290bbbc91042e9af5651e5f43671e  docs/tech-design  design/wake-channel-v1#design  docs/tech-design/wake-channel-v1.md
69b62a83828f3700a237206b3fa2de8d93cb563894e159d5d9cca74f004a6c36  docs/tech-design  design/wake-watch-v1#design  docs/tech-design/wake-watch-v1.md
a4ef3f21e48acae3a3df6c97dace8f9d87314a168ed1f21bde4482b1d622f9fd  docs/tech-design  design/wake-watch-v2#design  docs/tech-design/wake-watch-v2.md
4104e74727037b9480b28fd396d308271e73b58d249e6a729c6041a9ca8e4f46  docs/tech-design  design/wake-watch-v3#design  docs/tech-design/wake-watch-v3.md
5bd6e210a647b91ad19849731c61b8d4bf01344d41daeec11d27a993b8ecfd47  docs/tech-design  design/workbench-ui-v1#design  docs/tech-design/workbench-ui-v1.md
8df30463c23e9931b2ba4347e5472a6bc7cd89080820b691c5e46dd611efb6d6  docs/tech-design  design/worktracker-hierarchy-index#design  docs/tech-design/worktracker-hierarchy-index.md
449e6450db57626fa37c22b907d0667157ba1122d8c0c425eb901888bebcd7a5  docs/root  rollup/technical-spec#current  docs/techspec.md
340e2afb2c31e34ffbe9abff3803fdc98ed9a425299deac8ac153141565ceb62  docs/wiki  wiki/index#index  docs/wiki/README.md
ee93939db36212b4418cd4d7419877b3b87cf60b78d79cc407fe525b0ca6e20c  docs/wiki  wiki/document-classes#entry  docs/wiki/document-classes.md
d2e38c7ca33e75484271810ed963db11a4fca5f68c522a224d4e5d9993c9a564  docs/wiki  wiki/e2e-authoring-reference#entry  docs/wiki/e2e-authoring-reference.md
3573ef4c9e0ae4638452fbd88f0827d3ffac10ca7ef991ce5e5e22a3020ab0f2  docs/wiki  wiki/github-agentic-setup#entry  docs/wiki/github-agentic-setup.md
4dc67e0faddbafc056cd11a0be37733a5aef9ddc5f70109c2eb9e531a59d862a  docs/wiki  wiki/glossary#entry  docs/wiki/glossary.md
ffbfb54ec2b0b1f40032158cc137200807e863900414ff6ef3ccf635fd1967a1  docs/wiki  wiki/impl-review-reference#entry  docs/wiki/impl-review-reference.md
6af6ca526873f3a2911a57608cf8bc853c9694dd62b8d32953fdcf6b35285472  logs  session-log/2026-07-30-1024-planner-routing-t1#record  logs/2026-07-30-1024-planner-routing-t1.md
211346b355a12e8b73c6b08b7f9d1bfa2ba16b563dff02d2d7c5aab153a496e7  logs  session-log/2026-07-30-1030-planner-routing-t1#record  logs/2026-07-30-1030-planner-routing-t1.md
8d8183b9b8b8e944e7217812f76ebe097aeb56402692b4a79e005b6698a3a3c2  logs  session-log/2026-07-30-1033-planner-routing-t1#record  logs/2026-07-30-1033-planner-routing-t1.md
7fa563ec9a128e20e561394cc028dc4f20452361e0029d680964b9b3798784f5  logs  session-log/2026-07-30-1335-t1-reader-port#record  logs/2026-07-30-1335-t1-reader-port.md
80fcba7715e6b31cd595e3ae18936e220438db18113e1b89113a51e14b0a86b5  logs  session-log/2026-07-30-1352-t1-reader-port-reverify#record  logs/2026-07-30-1352-t1-reader-port-reverify.md
3c731ecad717d4c1e0003b2f68a867d3a283a3768b5dee291f3e8b528c001647  logs  session-log/2026-07-30-1355-planner-routing-t2#record  logs/2026-07-30-1355-planner-routing-t2.md
ef3f04370e79652f851fc10949eaf0f8b93a454d8fcf292a3445cabf83469572  logs  session-log/2026-07-30-1405-t6-wake-primitives-verify#record  logs/2026-07-30-1405-t6-wake-primitives-verify.md
28a85ec1ec5a23a4e2d4442d37c04c8b3dced0e4b45fa2422e39d76d8bc4183d  logs  session-log/2026-07-30-1506-github-adapter-batched-read#record  logs/2026-07-30-1506-github-adapter-batched-read.md
67e1bb135b05279af6f6533e2d2dd12c96783fec753cf412e4317fe1a910f179  logs  session-log/2026-07-30-1506-shared-write-budget#record  logs/2026-07-30-1506-shared-write-budget.md
910f4d6d56a143b772a1c4f2b920dc5642a832f8f49f80b4486d98aa194e051a  logs  session-log/2026-07-30-1506-t7-watch-loop#record  logs/2026-07-30-1506-t7-watch-loop.md
ef4418f2ca72a75e9a36a617b62b2a0a171a07cc58e2079c06b6aea3209b67db  logs  session-log/2026-07-30-1510-github-adapter-batched-read-t3#record  logs/2026-07-30-1510-github-adapter-batched-read-t3.md
f5bc8d5fc3ead5fb580a0a8e075aeef1c4620c1ab92fdcabaf39d8db5a2a6ab8  logs  session-log/2026-07-30-1511-shared-write-budget-t5#record  logs/2026-07-30-1511-shared-write-budget-t5.md
4e489cf1e00188fa3dbc963201fcae723f5527fb74d131c8c8bb675e7418304b  logs  session-log/2026-07-30-1519-github-adapter-batched-read#record  logs/2026-07-30-1519-github-adapter-batched-read.md
e55bcb6b6026caed1d75cccf70b307f4ba6cc72a1cfe5f5e4651751b55f71f8b  logs  session-log/2026-07-30-1519-t7-watch-loop#record  logs/2026-07-30-1519-t7-watch-loop.md
661bb3775e966d91e4dd6215c59407e0ce4e4a3837cba48707a8b4633d2fab07  logs  session-log/2026-07-30-1523-github-batched-reader#record  logs/2026-07-30-1523-github-batched-reader.md
eef27a55aae0501cb9e1c4802479ee13b669c152399bba2a5485cb29f5fc23fc  logs  session-log/2026-07-30-1525-shared-write-budget#record  logs/2026-07-30-1525-shared-write-budget.md
a6e2685aa82cab6596089feabe89c680e67e8c5530af95f81b77298f73da595b  logs  session-log/2026-07-30-1528-t2-carina-statereader#record  logs/2026-07-30-1528-t2-carina-statereader.md
72fbf3ccbb88fdeba71999d31f9e283dc19b58b30c3db9178128fb9a3c1ee76b  logs  session-log/2026-07-30-1534-shared-write-budget#record  logs/2026-07-30-1534-shared-write-budget.md
410c330d3fcd554cbbb9304629f9da99f4b4f20523a8344a57e64df5a2316b00  logs  session-log/2026-07-30-1548-brainstorm-question-legibility#record  logs/2026-07-30-1548-brainstorm-question-legibility.md
ee83c2e2b6769b164d994a0c69358dddb6c29904b8cbac9a91b217a6ee86cca6  logs  session-log/2026-07-30-1609-t4-single-source-check-derivation#record  logs/2026-07-30-1609-t4-single-source-check-derivation.md
39afb308d5589bd1fb1c83262ee3da6eb14afe4a8569e43ee56dc489fa48a879  logs  session-log/2026-07-30-1640-t4-seal-demand-capsule#record  logs/2026-07-30-1640-t4-seal-demand-capsule.md
24e945f47a719d151e2935c77922967bef8477f84e84fe6a120769b32f924be1  logs  session-log/2026-07-30-1720-t4-checks-reverify#record  logs/2026-07-30-1720-t4-checks-reverify.md
48db2def14f4345e19debf4a6d7ec2186dc77646b34705f317dd5b14463dad1c  logs  session-log/2026-07-30-1746-agents-md-session-log-rule#record  logs/2026-07-30-1746-agents-md-session-log-rule.md
f56f870ca932225deadd1af8ad5685c9c6ef22d475e8dbf96895cdf09422aff2  logs  session-log/2026-07-30-1749-e2e-read-cap-retune#record  logs/2026-07-30-1749-e2e-read-cap-retune.md
bfde2e5e18ed296d6068e071f5f0a3ec0e239bb68125a195b3d93eace8207817  logs  session-log/2026-07-30-1836-status-verb-t9#record  logs/2026-07-30-1836-status-verb-t9.md
fdf6cca552bcbf630602723017ffe6d7518dfead43a964049502a5ac81574ac6  logs  session-log/2026-07-30-1838-e2e-design-plan-new#record  logs/2026-07-30-1838-e2e-design-plan-new.md
e9575a4d0d7a68b0f5e4ce6e5679f617f30846e8452c391b50dd0163f9f7ec32  logs  session-log/2026-07-30-2138-capsule-review-fixes#record  logs/2026-07-30-2138-capsule-review-fixes.md
e6e5711a64185aa491c67248011caeb961c6cb343566699e7c4fcff83283e390  logs  session-log/2026-07-31-1023-native-routing-policy#record  logs/2026-07-31-1023-native-routing-policy.md
3fb5ef33c9c703fbf310e67a9768a422e21c8f9bdb3bc62d6c87ee0a15155676  logs  session-log/2026-07-31-1224-implement-now-self-drive#record  logs/2026-07-31-1224-implement-now-self-drive.md
514ae908da4555775405adc993178fb6657b896ce131f7ee76444feb5ff7c48f  logs  session-log/2026-07-31-1301-product-vision-memory#record  logs/2026-07-31-1301-product-vision-memory.md
a14d125b8f9933b1863ab2c45d50c6d6c58f2a37401b04071cfadbd86da30a75  logs  session-log/2026-07-31-1529-t1-routing-validator#record  logs/2026-07-31-1529-t1-routing-validator.md
94ad5359fc3d96e40e33183257877a4f69134ce3c2f42ecee3afed868afef813  logs  session-log/2026-07-31-1739-t2-native-route-resolver#record  logs/2026-07-31-1739-t2-native-route-resolver.md
3819d8b1aa287f853eea0f15c41f9de41109d64c610b319fea00b0c4e244f7dc  logs  session-log/2026-07-31-2121-t2-routing-resolver#record  logs/2026-07-31-2121-t2-routing-resolver.md
cb81eb087b5173309c9f3ebff0fef91f64fde8a8c4df776b37adcb4a95513f99  logs  session-log/2026-08-01-1528-e2e-design-plan-new-round2#record  logs/2026-08-01-1528-e2e-design-plan-new-round2.md
a638ab93a13623906d8c3548678ee71ba959896773141dc6ec6a9c22de78f6a0  logs  session-log/2026-08-03-0951-t3-catalog-override#record  logs/2026-08-03-0951-t3-catalog-override.md
e9a79c879b30dcae9c69950945432542ec79427a6561a3640e6371829dffe4b7  logs  session-log/2026-08-03-1107-t3-catalog-override#record  logs/2026-08-03-1107-t3-catalog-override.md
cb749ed6d41813a2d4d8aa571476344967919941d513f63d6e8f3e78fa073d51  logs  session-log/2026-08-03-1120-t3-catalog-replacement#record  logs/2026-08-03-1120-t3-catalog-replacement.md
4bdd95244b0bb286ad12f8ccf7f8996359d59b2128acfe41aaa1b66579a0a13f  logs  session-log/2026-08-03-1133-t3-catalog-replacement#record  logs/2026-08-03-1133-t3-catalog-replacement.md
8202dd1e732a5b8734486fe6c15608d9c7f495f1984f79e3c6da9d8ebeae9096  logs  session-log/2026-08-03-1439-e2e-design-plan-new-round1-fixes#record  logs/2026-08-03-1439-e2e-design-plan-new-round1-fixes.md
8a7b7e22219c06218664af278c5ce716b5a0db0403ddfd475fc014f5dd33f34e  logs  session-log/2026-08-03-1740-e2e-plan-plan-new#record  logs/2026-08-03-1740-e2e-plan-plan-new.md
1906c9680dee31a54891502b63f26a965fbe5820802256dfdb3a3319b36f6f25  logs  session-log/2026-08-03-1817-e2e-plan-plan-new#record  logs/2026-08-03-1817-e2e-plan-plan-new.md
4efac7820bfeec4e73f9507f94bb45ee2d59ebf9f2f433b016b43a8b4f2d43ce  logs  session-log/2026-08-03-2116-e2e-design-tech-design-brainstorm#record  logs/2026-08-03-2116-e2e-design-tech-design-brainstorm.md
d1acd3f14a9d7eade4c2d9467b684f684269d46cfc6d9d1d47dbebc671874f08  logs  session-log/2026-08-04-2010-e2e-plan-tech-design-brainstorm#record  logs/2026-08-04-2010-e2e-plan-tech-design-brainstorm.md
d561da9a00e678f9e24accdbeeb0913915dfe5345c213ad9ab2121a7d5790ece  logs  session-log/2026-08-05-0117-executor-turn-cap-push#record  logs/2026-08-05-0117-executor-turn-cap-push.md
4f483ff88e10d40ba305de3c24bb41d237031c1f3d028f61586e039c89db6027  logs  session-log/2026-08-05-0129-tech-design-brainstorm-implement-now#record  logs/2026-08-05-0129-tech-design-brainstorm-implement-now.md
9410c7afec4d3d96f8b70cf862616f32e964f1fb76926006e07f4885880b5003  logs  session-log/2026-08-05-0623-worker-budget-1h#record  logs/2026-08-05-0623-worker-budget-1h.md
b742499cbee3b08408e3852a9ca4ccd205f008421dfc288fa61981c706e603eb  logs  session-log/2026-08-05-0636-tech-design-brainstorm-e2e-drive#record  logs/2026-08-05-0636-tech-design-brainstorm-e2e-drive.md
eb401dc1449ef85ffc4259623aae60e2786b46a6504581c94c5a77544cf25af1  logs  session-log/2026-08-05-0639-tdbrainstorm-e2e-harness#record  logs/2026-08-05-0639-tdbrainstorm-e2e-harness.md
6d1628fa15fcf604044815f3da6e86dcfd83f2c51a5bc8d3742e3696259d1896  logs  session-log/2026-08-05-1213-t20-non-regression#record  logs/2026-08-05-1213-t20-non-regression.md
377b4807c959384367f00a3ebd71e26987d65cf2ef3bfc1b8e552a61bb04b0fd  logs  session-log/2026-08-05-1214-t18-record-class-conformance#record  logs/2026-08-05-1214-t18-record-class-conformance.md
b1eca374a43b7456ac443a4ed44456bac2aa61c1d7c2403f7cf7d2dad6f083b1  logs  session-log/2026-08-05-1214-tdbrainstorm-artifact-conformance#record  logs/2026-08-05-1214-tdbrainstorm-artifact-conformance.md
e160ebddaac6c3b0fb56c26477cb639f0b7619bfd5f0de61e454678911e5c14f  logs  session-log/2026-08-05-1228-tdbrainstorm-e2e-handoff#record  logs/2026-08-05-1228-tdbrainstorm-e2e-handoff.md
f27b9e7b0c262199005c1818e053a8d146d5f399c55c5d31b0281e94b312daa0  logs  session-log/2026-08-05-1230-t16-write-perimeter#record  logs/2026-08-05-1230-t16-write-perimeter.md
cc7660a88cb541b958dca6beca687a9a6c31c38d2aea967cb437904651d4c7c3  logs  session-log/2026-08-05-1232-tdbrainstorm-glossary-collision#record  logs/2026-08-05-1232-tdbrainstorm-glossary-collision.md
80314fda5e96930948d0c2be8c093d3128b172ee78d09c2891605b11e3a4aad4  logs  session-log/2026-08-05-1233-t14-question-form#record  logs/2026-08-05-1233-t14-question-form.md
ca83b93178ad39a77ab2948e8ab7e8d2cb596096e390a6bd261f56ab065f46a2  logs  session-log/2026-08-05-1253-t13-reopen-scenario#record  logs/2026-08-05-1253-t13-reopen-scenario.md
a611f913b20672acdd3acaddeb9aff5030616c24e5e09c28fc17ca328c5889ee  logs  session-log/2026-08-05-1254-t12-kill-and-resume#record  logs/2026-08-05-1254-t12-kill-and-resume.md
f576767eb62553c443aa9420570413483b77d6cba9cae024f58d7be9cd371a84  logs  session-log/2026-08-05-1256-t10-covering-design-ends-session#record  logs/2026-08-05-1256-t10-covering-design-ends-session.md
5393a2174cabdf8d39e6da8c1cfa837ebbc95785c5134b3c2357911192efed54  logs  session-log/2026-08-05-1256-t11-planted-instructions#record  logs/2026-08-05-1256-t11-planted-instructions.md
d77a4b27872fbc6805bfd8b79cfc3522dfe7ae4dfde1e7820b9b23f6b7b0eb24  logs  session-log/2026-08-05-1312-t9-corrupted-record#record  logs/2026-08-05-1312-t9-corrupted-record.md
8ed4865587ba1423ddb99d6ec1df2865586bfc5d984191a25c43c63a1a2dc6bc  logs  session-log/2026-08-05-1319-cursor-role-catalogs#record  logs/2026-08-05-1319-cursor-role-catalogs.md
dbe4865268f2dc4dc23ff8b02a0d4ef0fd24fbb97eaa41481e58c6cff9cae70b  logs  session-log/2026-08-05-1319-t4-codex-role-catalogs#record  logs/2026-08-05-1319-t4-codex-role-catalogs.md
c9166fe5edf20c772648c5de03559ab5d2d4452073bd2607175e3b030f0d5afb  logs  session-log/2026-08-05-1319-t5-claude-routing#record  logs/2026-08-05-1319-t5-claude-routing.md
af6696c069aef344848a64e9f89ec4c7a49f5b07e28ac6ab3ee303b21c038753  logs  session-log/2026-08-05-1319-t7-opencode-role-catalogs#record  logs/2026-08-05-1319-t7-opencode-role-catalogs.md
c2e4f4a7eb9fed3e60906a3575800c1c1523dc7a73495ec5c98c547405ad6150  logs  session-log/2026-08-05-1324-tdbrainstorm-e2e-contradiction#record  logs/2026-08-05-1324-tdbrainstorm-e2e-contradiction.md
2a6f02b943cfb66360c033a278db15255113dfd5be4c06f10d4a5805518e1e70  logs  session-log/2026-08-05-1326-t7-silence-scenario#record  logs/2026-08-05-1326-t7-silence-scenario.md
5baef3015725c2726189be4542c7e65bba872793693175bf072773d49105d460  logs  session-log/2026-08-05-1339-t6-under-specified-fog#record  logs/2026-08-05-1339-t6-under-specified-fog.md
7249b1ee0c51eaf252103e0396126815e92b0c97c0e2d4dad849c5fbd0e0dc2e  logs  session-log/2026-08-05-1340-tdbrainstorm-bounce-scenario#record  logs/2026-08-05-1340-tdbrainstorm-bounce-scenario.md
a1f5842ced03ff9d50a6c3766ccef7e61d70029b882e0184fd0e7fafbf6a3673  logs  session-log/2026-08-05-1349-t4-panel-discrimination#record  logs/2026-08-05-1349-t4-panel-discrimination.md
892e24d15431f112391588d61b4be9776094726b8b3e6f04e17876f46ae84499  logs  session-log/2026-08-05-1358-t10-covering-design-ends-session#record  logs/2026-08-05-1358-t10-covering-design-ends-session.md
da30621f42dbc4d8bff92af32b0d0d91df44ec04d712d24e2c7f05db07c7f82d  logs  session-log/2026-08-05-1359-t3-golden-session#record  logs/2026-08-05-1359-t3-golden-session.md
b9e4affaf92dd937432ae8f522756dad6987ad37d6a75ab78d2f6da533d6305d  logs  session-log/2026-08-05-1401-tdbrainstorm-basis-gate#record  logs/2026-08-05-1401-tdbrainstorm-basis-gate.md
7aff72aace59c88e681eb6fc8e1359c643d4eaa77c9522c9f9e630629c43ea69  logs  session-log/2026-08-05-1454-t8-routing-domain#record  logs/2026-08-05-1454-t8-routing-domain.md
756f2fe01f9aef09c09927826fbef9e6ded43b49bb3334586e5556fcef71cfb7  logs  session-log/2026-08-05-1527-t9-routing-acceptance#record  logs/2026-08-05-1527-t9-routing-acceptance.md
5f3f21c3de77a4afb5754e405751f1f2044a25cfd76490d5a66168e9ae491897  logs  session-log/2026-08-05-1901-t11-impl-review-fixes#record  logs/2026-08-05-1901-t11-impl-review-fixes.md
cace77f7f6fa218c0ba60507a968ad7b7db9f151956d40f059321a283b43f8fb  logs  session-log/2026-08-05-1904-t20-impl-review-findings#record  logs/2026-08-05-1904-t20-impl-review-findings.md
8937dcf4514491fe8b58d9a1e59982a81faa059fc44d9c1781a38a9c75c181aa  logs  session-log/2026-08-05-1910-t14-impl-review-findings#record  logs/2026-08-05-1910-t14-impl-review-findings.md
0c14486ec516dcc94bcb40f3abe814e6ec9773cb0de5d4928234f1f9395c912d  logs  session-log/2026-08-05-1911-t7-impl-review-fixes#record  logs/2026-08-05-1911-t7-impl-review-fixes.md
e377ba3504fe45b55562edc934a4e3ec6014010db09c915139655fd50805c707  logs  session-log/2026-08-05-1922-t16-impl-review-fixes#record  logs/2026-08-05-1922-t16-impl-review-fixes.md
89c443a759a8ab0155b858207ea0b7fef0493c930f7cf4f489695ef104d5b0ca  logs  session-log/2026-08-05-1928-t12-impl-review-findings#record  logs/2026-08-05-1928-t12-impl-review-findings.md
7a6d37cb6bb5ea32b485388a0f76b2b37339e583e13673b061ed6104ca4b9367  logs  session-log/2026-08-05-1928-t8-contradiction-review-fixes#record  logs/2026-08-05-1928-t8-contradiction-review-fixes.md
486d3b495facaae2c74c92be23dcb28e9c4c88aaffdacc9889cbb0f1e1b68604  logs  session-log/2026-08-05-1939-t17-impl-review-findings#record  logs/2026-08-05-1939-t17-impl-review-findings.md
fa5292b97a1aa318d5f6c2226ac6010496a69f5cda507444effdd386f9dce767  logs  session-log/2026-08-05-1950-t3-impl-review-fixes#record  logs/2026-08-05-1950-t3-impl-review-fixes.md
127f52c89c7af8fa5e9729b8ce38ea726d46e9996abc5081e0be13fcf13b5d43  logs  session-log/2026-08-05-1953-t13-impl-review-fixes#record  logs/2026-08-05-1953-t13-impl-review-fixes.md
738e6789f67799bc61530d6d9d868de0d7be92022caa26f705890a6bd14e342b  logs  session-log/2026-08-05-1953-t6-impl-review-fixes#record  logs/2026-08-05-1953-t6-impl-review-fixes.md
cfafa0f03304c1395afac4ac16f9aedb49ee1722dbebab7633efcf708f63e57d  logs  session-log/2026-08-05-1956-basis-gate-review-findings#record  logs/2026-08-05-1956-basis-gate-review-findings.md
48fc25edc3b5ff906f31ec51baac152bdce304cad482b081f9d81cff48342d2f  logs  session-log/2026-08-05-1958-t9-impl-review-fixes#record  logs/2026-08-05-1958-t9-impl-review-fixes.md
6f1b0ef9338c1021f4db4027a61252a43d1bca6378d2eca057f3998b3d99441a  logs  session-log/2026-08-05-2002-pr-1006-impl-review-findings#record  logs/2026-08-05-2002-pr-1006-impl-review-findings.md
2908f3f3884d68623f44fc89b93713c6fb773526b2568dbf9a1aefe3150a07d9  logs  session-log/2026-08-05-2004-t4-review-findings#record  logs/2026-08-05-2004-t4-review-findings.md
c8b698dd506c4b4bee2a9d64e2ab5044f721678dd7e5f8611c99c9498ed03f6c  logs  session-log/2026-08-05-2006-t5-impl-review-fixes#record  logs/2026-08-05-2006-t5-impl-review-fixes.md
640ec9d083e8764160dc6287f9cce1436a0cfd29c0c30a70dd130cf70a3b8c17  logs  session-log/2026-08-05-2010-pr-1011-impl-review-findings#record  logs/2026-08-05-2010-pr-1011-impl-review-findings.md
b7a31fdf865f7e1a67b728f43fd6b2246dcab708c736cd023c5fe1b1e1c46b09  logs  session-log/2026-08-05-2012-t10-impl-review-findings#record  logs/2026-08-05-2012-t10-impl-review-findings.md
3223dfe19c2c2835dabd5ebf5b86ab38a8e1b67e4e25acb0408b6dae41ea4d67  logs  session-log/2026-08-06-0057-e2e-test-symbol-collisions#record  logs/2026-08-06-0057-e2e-test-symbol-collisions.md
3f2da1a1ebcd693eb30bcdc8bb493c226d243c13d3c7782a02d40cc3b5ed3eb6  logs  session-log/2026-08-06-0131-tdbrainstorm-e2e-run#record  logs/2026-08-06-0131-tdbrainstorm-e2e-run.md
71455f3456f44e8c63229aecb2a723096c26c61ab47b23eb8e35f243615bb44b  logs  session-log/2026-08-06-1717-e2e-design-gaps#record  logs/2026-08-06-1717-e2e-design-gaps.md
cf202dc6754c53f0a13cbec764a466f8b8e38d7159ba2cf5e90c4a139d843e25  logs  session-log/2026-08-06-1817-executor-journal-gitignore#record  logs/2026-08-06-1817-executor-journal-gitignore.md
797de0957e43ccae3b5924ef7c3ed32a7212c83f0c758446b98fbcf5c6a45873  logs  session-log/2026-08-06-1939-e2e-verification-design-brainstorm-reopen#record  logs/2026-08-06-1939-e2e-verification-design-brainstorm-reopen.md
733e9212a587602bf4939fac9c1350c3c8e19d8968a587df0f38434a1316f6f6  logs  session-log/2026-08-07-0000-e2e-t2-buildability-schema#record  logs/2026-08-07-0000-e2e-t2-buildability-schema.md
d7a3d23eadd69536e0af006e609b5f256664f2cc1f8715af20b84d420692869d  logs  session-log/2026-08-07-1935-e2e-verification-002-implement-now#record  logs/2026-08-07-1935-e2e-verification-002-implement-now.md
b82156520391f5c49a475b6346fa54bd9f3f9693c01ee3662500c15dbf9af702  logs  session-log/2026-08-07-1942-e2e-design-v2-class-and-checker#record  logs/2026-08-07-1942-e2e-design-v2-class-and-checker.md
c4eab392ee79aeb997abcf6d7b6cc50515cab651d061cd7705e1e4ed33bb4887  logs  session-log/2026-08-07-1942-t2-buildability-finding-schema#record  logs/2026-08-07-1942-t2-buildability-finding-schema.md
931df760808aede8c8b8b1ed59028f4c274f7e4690c61d3a354a5d8aa7470a27  logs  session-log/2026-08-07-1944-t2-buildability-lens#record  logs/2026-08-07-1944-t2-buildability-lens.md
7eaeffa4ab2368b7d9642508178b8911638872fee5e4720b6d21f9817a839740  logs  session-log/2026-08-07-2107-impl-review-1075#record  logs/2026-08-07-2107-impl-review-1075.md
bfc3a5760a78191b7770dc59b680d1ddade1fbacb8cffca53051e887b788f629  logs  session-log/2026-08-07-2107-impl-review-and-finding-resolution#record  logs/2026-08-07-2107-impl-review-and-finding-resolution.md
2729380427f7e9cf8d6b5f7304f02a91afc2bcb511b50faddc6a7a2eca13fa33  logs  session-log/2026-08-08-0000-t3-harness-form-picker#record  logs/2026-08-08-0000-t3-harness-form-picker.md
436823008245e5fcb99cd1f7776fdfd85633d5accf5d156764893ac1cabd13e8  logs  session-log/2026-08-08-0047-e2e-verification-002-drive#record  logs/2026-08-08-0047-e2e-verification-002-drive.md
b1e41f4df7aef28667098250d8bb374f444e958b17dc751aecc39668a4ed160a  logs  session-log/2026-08-08-0050-t7-buildability-blind#record  logs/2026-08-08-0050-t7-buildability-blind.md
a085b88da33ea70fd645fc1c708fa4f57088e2e1223e8571aa6174b9a372a6ba  logs  session-log/2026-08-08-0107-t3-harness-form-gofmt-fix#record  logs/2026-08-08-0107-t3-harness-form-gofmt-fix.md
59346987bf65c244ef42204a2b60d2c3d6fa04c07ed74c1d709cf4a59c6bc34b  logs  session-log/2026-08-08-0410-impl-review-epic-1061#record  logs/2026-08-08-0410-impl-review-epic-1061.md
765a3b9cab3fb63123e9dc796aea97b9be3a39e4812d736bf499428681f4ceb1  logs  session-log/2026-08-10-1015-e2e-verification-002-t9-implement#record  logs/2026-08-10-1015-e2e-verification-002-t9-implement.md
c132c1cb6e37f1574f42467fd2a1e7fe59975150df9ef5040f31e7dcfa1f3f20  logs  session-log/2026-08-10-1400-t5-e2e-design-decision-authority#record  logs/2026-08-10-1400-t5-e2e-design-decision-authority.md
42fafc8af118ffad3fab9f006a5e9dd817a3f98a86e705c0b7325066ae33d1e3  logs  session-log/2026-08-10-1735-e2e-verification-002-drive#record  logs/2026-08-10-1735-e2e-verification-002-drive.md
ea8caa9dfc94798e44ff03a69bbb401c01242e48e44ed61e944a83436606e9a7  logs  session-log/2026-08-10-1737-t4-scenario-normalization#record  logs/2026-08-10-1737-t4-scenario-normalization.md
e9a203ab9ae4d7aa9588066f0080139c2241b327e73a3c57c0518ff7a02e57ec  logs  session-log/2026-08-10-1738-t11-corpus-docs#record  logs/2026-08-10-1738-t11-corpus-docs.md
1778e840853a32e468d5c04dcd0a8331d5dab2371442d2ac85b00807fcecc458  logs  session-log/2026-08-10-1747-impl-review-epic-1061#record  logs/2026-08-10-1747-impl-review-epic-1061.md
4b1a2494a2ebab5f2832b2e8f0996f4312f38f44afcea3e57e8f85abe3fe41a0  logs  session-log/2026-08-10-1940-t5-completion-verification#record  logs/2026-08-10-1940-t5-completion-verification.md
ba1acbeb8413021cf4f7cdb73feaf64de1478156c87f3bc72f1fd00ff9e6cdd2  logs  session-log/2026-08-10-1950-impl-review-1083-1084#record  logs/2026-08-10-1950-impl-review-1083-1084.md
8a42de248f89f17e7b042f1a1d06d09efbcab98b0637013bd2afdecf62e4bed9  logs  session-log/2026-08-10-1951-t9-e2e-plan#record  logs/2026-08-10-1951-t9-e2e-plan.md
9de9c22de2afa8a96bc43709c76f87a9f2c13fac3d421dc5814de35f8fb7e5c6  logs  session-log/2026-08-10-2253-t10-e2e-plan-refuses#record  logs/2026-08-10-2253-t10-e2e-plan-refuses.md
31769cf14d693e493ab3d7476ac4b52725b2b30d0e123d836afd9a8f916b8d04  logs  session-log/2026-08-10-2315-implement-now-epic-1061#record  logs/2026-08-10-2315-implement-now-epic-1061.md
58d61264143a075f71c4737dc5dce542fc0986877b4ebf6cae41316be2c4d91b  logs  session-log/2026-08-10-2319-t6-e2e-design-blocking-open-item#record  logs/2026-08-10-2319-t6-e2e-design-blocking-open-item.md
954ac62987f4786fe88cad10cb95478faa9e134712e799a65ee106445a779768  logs  session-log/2026-08-10-2326-impl-review-epic-1061#record  logs/2026-08-10-2326-impl-review-epic-1061.md
eeaa52512f1505cb138ed842593935ce66336ad60e462cb836261704c1eaea84  logs  session-log/2026-08-10-2330-pr-1087-ci-fix#record  logs/2026-08-10-2330-pr-1087-ci-fix.md
436e0bce9d156bf217adcedc23b0ed846cb2c92554cea09f7a324739d85a2222  logs  session-log/2026-08-11-0306-impl-review-epic-1061#record  logs/2026-08-11-0306-impl-review-epic-1061.md
ed1d7e5f8ef5a92fd3a089ab02da612c7d5eca3356b50e0df79447f6b2db723a  logs  session-log/2026-08-11-0313-pr-1087-ci-fix#record  logs/2026-08-11-0313-pr-1087-ci-fix.md
b24cb063d587446b391ecf3eeb089bb5d5b4c2178d624c46d8fdb3574de3201e  logs  session-log/2026-08-11-1400-t12-update-rollups-authoring#record  logs/2026-08-11-1400-t12-update-rollups-authoring.md
994892c96db4e67ff06a81f89c522bee478be2583b69a7fde3ccc45f099bb481  logs  session-log/2026-08-19-0934-summarize-skill#record  logs/2026-08-19-0934-summarize-skill.md
8d3343a3948fba7308fe04fd6e1670d48d351a39b3bb0d7cdc15cf57eaf7dd9c  logs  session-log/2026-08-19-1357-pr1055-session-trace#record  logs/2026-08-19-1357-pr1055-session-trace.md
b4625af798e7e071b1392bb0651c8ccdf14538ddd0093c84c76adc9d51e762f4  logs  session-log/2026-08-19-1523-pyxis-init-404-diagnosis#record  logs/2026-08-19-1523-pyxis-init-404-diagnosis.md
c5c44df10c2d520fdd2156048005a221662ce15db8969135360411c26901d603  logs  session-log/2026-08-20-1105-pr1092-event-join#record  logs/2026-08-20-1105-pr1092-event-join.md
64c68a173e07deb5c5e56dcea085655e8224cb5f017fbe7742f0cd2063ec4e62  logs  session-log/2026-08-20-1106-address-projection-gate-review#record  logs/2026-08-20-1106-address-projection-gate-review.md
b4b699bce97349d878945f1ee8739a1829ae7fa08805b61ed2f470d2da3ba893  logs  session-log/2026-08-20-1126-projection-congruence-upstream-review#record  logs/2026-08-20-1126-projection-congruence-upstream-review.md
14425c7fb73e513f8dee05e80e16de07ee29d228938b658e8a87e879105da67f  logs  session-log/2026-08-20-1128-pr1092-reciprocal-join#record  logs/2026-08-20-1128-pr1092-reciprocal-join.md
111406d1e7d995852eda80b71a3f4f2e5d0aad56e20add289f20aa603d6c9d09  logs  session-log/2026-08-20-1153-implement-now-amendment-006#record  logs/2026-08-20-1153-implement-now-amendment-006.md
505a8740cad76f386bd9b6a225d4fcf874a12e43e4564bf1f08591f87f0901a7  logs  session-log/2026-08-20-1345-public-install-endpoint#record  logs/2026-08-20-1345-public-install-endpoint.md
8472226851c45103eddfb286011e638610c3e2d0163ff622820e671506d4fc68  logs  session-log/2026-08-20-1431-projection-congruence-restoration#record  logs/2026-08-20-1431-projection-congruence-restoration.md
03a8df3a96d191d3e0fa017f542c2e7395354f412a7c1fe37d1c2242cd754d23  logs  session-log/2026-08-20-1527-prd-model-v2-design#record  logs/2026-08-20-1527-prd-model-v2-design.md
ba09b900b87f603ff236fa62c57300b456291a02c25fa36aa614277f6733a9a2  logs  session-log/2026-08-20-1530-prd-model-v2-coherence-review#record  logs/2026-08-20-1530-prd-model-v2-coherence-review.md
35d10d7ad7ffa1c6375506ef4c7b1c5ae53d5b2217b888789e5c56cd1a41a6d4  logs  session-log/2026-08-20-1530-prd-model-v2-upstream-review#record  logs/2026-08-20-1530-prd-model-v2-upstream-review.md
c2cae0cf4564a1e1422f77c7d685ccdfe0d1c1a59af1590bc788ac02416e68b8  logs  session-log/2026-08-20-1538-prd-model-v2-contract-review#record  logs/2026-08-20-1538-prd-model-v2-contract-review.md
933ecbf3a1fcde1b06286e39a47f89b4c19585da0b20bb06961d19302733a2f3  logs  session-log/2026-08-21-0951-pr1034-round5-fixes#record  logs/2026-08-21-0951-pr1034-round5-fixes.md
3ef18e95c5fcb48557cec85da861bf14eb4a6aa061939e8112c0737754ab1c35  logs  session-log/2026-08-21-1259-pr1034-round6-fixes#record  logs/2026-08-21-1259-pr1034-round6-fixes.md
5332f11914e0d02a7752b9ee7de63c1d54da3745da4c88e5c42087fc95f503d7  logs  session-log/2026-08-21-1700-routing-evidence-plan#record  logs/2026-08-21-1700-routing-evidence-plan.md
a99aa37656b78073ce9a346b77d18cccc6771d52c55b55eef675a5dda6c52258  logs  session-log/2026-08-21-1843-routing-evidence-t1#record  logs/2026-08-21-1843-routing-evidence-t1.md
beeb4302dabc098f2b3481b5b25a0f8dc75eec9a44a2484e461d39ac9a0d3697  logs  session-log/2026-08-21-1916-authenticate-event-producers#record  logs/2026-08-21-1916-authenticate-event-producers.md
462855037db00f1916727528c7be65dfde11cc9371d47b0b882591d79752e2a7  logs  session-log/2026-08-24-1029-routing-reducer-t3#record  logs/2026-08-24-1029-routing-reducer-t3.md
c0eb5bc8439dd1c6148ab5d8d729e32636676ccc280d7ae1bfea5356a34cd8cd  logs  session-log/2026-08-24-1139-harbormaster-watchdog-lessons#record  logs/2026-08-24-1139-harbormaster-watchdog-lessons.md
ed25efd9ae40bd12913688f63822ebbfb3b7bf587399f62822c81e2b60afea17  logs  session-log/2026-08-27-1220-t4-routing-evidence-append#record  logs/2026-08-27-1220-t4-routing-evidence-append.md
81841f15578206857e790f78ca41947bb66d183e0f6ab1a3af791e74ebd70602  logs  session-log/2026-08-27-1247-t5-routing-admission#record  logs/2026-08-27-1247-t5-routing-admission.md
d1405fe764f6ea49549e7b64ca3e646d77370ad3ef248d2553b6698f146562bf  logs  session-log/2026-08-27-1359-runtime-spawn-fencing#record  logs/2026-08-27-1359-runtime-spawn-fencing.md
e5896081115acdaba7628fbb57ca3e5bd125d5a9947515e1dc2c324cfab0e9dd  logs  session-log/2026-08-27-1454-t7-attempt-outcomes#record  logs/2026-08-27-1454-t7-attempt-outcomes.md
4d73a1055e0813ead087099b9c0a11b4fd3dc20b00a5c8d605b7eff6242e25aa  logs  session-log/2026-08-27-1537-t8-review-evidence#record  logs/2026-08-27-1537-t8-review-evidence.md
56881964038c4e58beadd04dee8db9f1c9a1c00d13fa05944c98a7df803a08b5  logs  session-log/2026-08-27-1614-t9-evidence-invalidation#record  logs/2026-08-27-1614-t9-evidence-invalidation.md
b6eee2651cc9f3e93f052ce657d943a50ddd28df37b15bb8b3ee5ac4080fc88d  logs  session-log/2026-08-27-1714-t10-activation-checkpoint#record  logs/2026-08-27-1714-t10-activation-checkpoint.md
39f5ac762651b4320211fbbd2abe22169cc0f2d103bf54b7af66adb64641745a  logs  session-log/2026-08-27-1746-t11-operator-faces#record  logs/2026-08-27-1746-t11-operator-faces.md
36f0ec88a105a921c533cb5907092fbab1a2e362d89bc54b24997f42cd346436  logs  session-log/2026-08-27-1826-t12-routing-evidence#record  logs/2026-08-27-1826-t12-routing-evidence.md
ccaf5c3aed42d0f1921433b7ea4999fc6dc6841deca3ecd2f6b6d0dba0e134fd  logs  session-log/2026-08-28-1024-routing-evidence-e2e#record  logs/2026-08-28-1024-routing-evidence-e2e.md
338a7e906de28ea0ce79c26f5da39596004c510242e9b7b24d9eae15628569c1  logs  session-log/2026-08-28-1202-routing-evidence-activation#record  logs/2026-08-28-1202-routing-evidence-activation.md
c82bd5cf4397cba1959ab89480b570dea40b591e21df31b76056c9d5408f6052  logs  session-log/2026-08-28-1413-implement-now-e2e-suite#record  logs/2026-08-28-1413-implement-now-e2e-suite.md
aa07836a138b0aeb6c401845d598eef8893de8ed40e819df9f6c329d52a2cbea  logs  session-log/2026-08-31-1447-coverage-gate#record  logs/2026-08-31-1447-coverage-gate.md
8c81ffda2152adedc3c7a564dbd4c2ea39ae75b116d5a8d6dfcd400e4b142aab  logs  session-log/2026-08-31-1557-executor-e2e-real-git#record  logs/2026-08-31-1557-executor-e2e-real-git.md
0040ac01d18614b6599515cf24e8ddf72f50c16afc2ad780808d16ace7160853  logs  session-log/2026-08-31-1723-implement-now-local-ci-gap-review#record  logs/2026-08-31-1723-implement-now-local-ci-gap-review.md
b967ed3af32139ae1d023f36e7f8b9b4a416a4c8f598fa6c0018d0070998d8cd  logs  session-log/2026-09-01-0951-coverage-gate-ci#record  logs/2026-09-01-0951-coverage-gate-ci.md
c211262ff85af6b756f88919e72ff299da1b463e9ed86c7d4d27d7b58cadba87  logs  session-log/2026-09-01-1005-coverage-gate-pr#record  logs/2026-09-01-1005-coverage-gate-pr.md
0329ce510394bde2eac7acf424ea75e43e6bd13ae7d235bbbc096da5bdf30f57  logs  session-log/2026-09-01-1057-docs-gate-fix#record  logs/2026-09-01-1057-docs-gate-fix.md
da7cccab7db1d6011bf1bf0a368de3d79abaed3b85d738b52d45819efeafe66d  logs  session-log/2026-09-01-1149-coverage-90-gap#record  logs/2026-09-01-1149-coverage-90-gap.md
1f855f204fc6960b896670db914d7c51a9fb41a262d8842c538c47a0e99241a7  logs  session-log/2026-09-01-1157-raise-coverage-threshold#record  logs/2026-09-01-1157-raise-coverage-threshold.md
a1f22e422d18c82d324db5693e9144612549b8527d3b55abe05aae459934b1a0  logs  session-log/2026-09-01-1211-coverage-90-pr#record  logs/2026-09-01-1211-coverage-90-pr.md
f08bf86cc83d406a689bd110a69dea9df8f368ec7216b4bb641813a0390ada65  logs  session-log/2026-09-01-1221-coverage-90-tests#record  logs/2026-09-01-1221-coverage-90-tests.md
9d3b35dff71528706674dc471c076a1946faf2706cc3842508755a8f61fe41d2  logs  session-log/2026-09-03-1706-pr-1162-fixes#record  logs/2026-09-03-1706-pr-1162-fixes.md
00be6ace2eb2df60fab8480e0946de07c3a739919892e60a4f1353fc378c2f01  logs  session-log/2026-09-03-1707-document-integrity-fixes#record  logs/2026-09-03-1707-document-integrity-fixes.md
c13922005f0e186ac993139660f97323aad888d2f328fd48b2cc4c8140399498  logs  session-log/2026-09-03-1707-memory-doc-fixes#record  logs/2026-09-03-1707-memory-doc-fixes.md
430db076e29ecd5b97058b8fbb135b8c6f78c1f8f65a7ddfbf33cdaa77ef7f39  logs  session-log/2026-09-03-1707-pack-fixes#record  logs/2026-09-03-1707-pack-fixes.md
ff087d2894f4821fa19f4ee6c9c98abf73291bc5a2bc79b418b685de85d18d51  logs  session-log/2026-09-03-1707-packet-fixes#record  logs/2026-09-03-1707-packet-fixes.md
3a2796c2ebcea3288a8c68ad39a2d3e0f59c4bc9d0599fb958911c5ca437613f  logs  session-log/2026-09-03-1717-routing-simplification#record  logs/2026-09-03-1717-routing-simplification.md
e2a63b745c5ace0f20d02c25d9b5b775deefec2271d67dbc4ae180f28abc06b9  logs  session-log/2026-09-03-1729-routing-resolver-tests#record  logs/2026-09-03-1729-routing-resolver-tests.md
46a67b4c097ae268e9d4c987dbacee1a8776a06d06c520ee09d4f0f0c1d42f6c  logs  session-log/2026-09-03-1734-routing-telemetry-tests#record  logs/2026-09-03-1734-routing-telemetry-tests.md
55a690f0ec501fa8230576018dc31cc65fcd4729550be861c9670e5bfeedce9c  logs  session-log/2026-09-03-1804-resume-stickiness-review#record  logs/2026-09-03-1804-resume-stickiness-review.md
60b3bb244636d7992c4268fc3430de6d7dc4340e24a0b655cf0150015179dddd  logs  session-log/2026-09-03-1808-pr-1162-conflict#record  logs/2026-09-03-1808-pr-1162-conflict.md
f72abdf20f4e887037eb2434c476c6f3696c18bcab590b385b257c71831f7369  logs  session-log/2026-09-03-1811-routing-final-review#record  logs/2026-09-03-1811-routing-final-review.md
d34abe0d884fd1633761cd6a1cd888f39b1228d16cf8b1549b34a32862171bc0  logs  history/session-logs#index  logs/README.md
```

## Exclusion inventories

### generated-derived

```text
mode  git-object  source-path
100644  20812fa05c8d5d8bc15e752f1ef583ebcd0dd7ca  .pyxis/generated/harnesses/claude/skills/implement-now/SKILL.md
100644  b7c73a41fb62854a0fcb66265caaefc1531ce9f7  .pyxis/generated/harnesses/codex/skills/implement-now/SKILL.md
100644  99eb97118ba224bc1ccb50931f82c3f44f9a2701  .pyxis/generated/harnesses/cursor/skills/implement-now/SKILL.md
100644  b44732559b76a1bc3fe9ef10f2fe597f457defa7  .pyxis/generated/harnesses/opencode/skills/implement-now/SKILL.md
100644  fd6d9dbc1ea581bb1afb94d3c6ff27239c3c2d03  docs/feedback/002-plan/effective.source.json
100644  3e50bb4761006c709b38ba51fbbeff51c22d925f  docs/plans/argus-boundary-enforcement/001-plan.json
100644  b3970db652e3ab8626e6230030bdec0084238d3c  docs/plans/argus-boundary-enforcement/002-plan.json
100644  7f107b6a5d28f0385e1f872130d8a29c8b52bfa5  docs/plans/argus-ledger-lint/001-plan.json
100644  623676d793c15271380de432bc97f0c9966d5695  docs/plans/argus-unattended-drive/001-plan.json
100644  fd7c6d84d940cb46b614e4c1c8b4dcfd4c75bb36  docs/plans/config-driven-portability/001-plan.json
100644  3ccc41f1fd29d0ab07c686b7d4bc2f0f0afe225c  docs/plans/context-pack/001-plan.json
100644  700460214addf80977c8ec6c5ba917526aef2771  docs/plans/contract-gate/001-plan.json
100644  e7a3d35f6165c49c7273f8c49efb062ab389c90a  docs/plans/design-solution-shape/001-plan.json
100644  909923d991c04c716952322bad800ed11ed7a6ac  docs/plans/e2e-verification/001-plan.json
100644  7793bda99ba3154d93ec5028514847e873b64ad8  docs/plans/e2e-verification/002-plan.json
100644  d859f66a0a9732d06f88abed82df7b753b018351  docs/plans/ecosystem-help/001-plan.json
100644  d1c5ec4a2900818713ad91ec61e2cfe5ffa35f30  docs/plans/existing-repo-onboarding/001-plan.json
100644  dbc30518eb304e393489fed6ed14ced2b1ae3c9a  docs/plans/feature-implementation-drive/001-plan.json
100644  4d787a0dc17035df58905ceb958cd52afa0bfa0b  docs/plans/impl-review/001-plan.json
100644  b33b6eacc04b101c767b0466f45fbdb8ffafbad4  docs/plans/implement-now/001-plan.json
100644  3b06031f1c4178b49114958e8b501c9b42b2af8b  docs/plans/implement-now/002-plan.json
100644  190f5695dd17455508ba74c5c408fa519e06854d  docs/plans/implementation-verification/001-plan.json
100644  62f06845d862d7a50c319ecf77d0791729150dcb  docs/plans/init-hardening/001-plan.json
100644  52bdfa38b82977af7de5f84faf30a01179544a9b  docs/plans/installer/001-plan.json
100644  4069012869ac0fdac4b4ad7ac333135d786e9435  docs/plans/local-work-tracker/001-plan.json
100644  e3c4fde3c55addcaf91ab3a9374a002a5ea13c1e  docs/plans/local-work-tracker/002-plan.json
100644  4af8d68f60980b9e7e1b203a678f5bd797075835  docs/plans/maintenance-sweep/001-plan.json
100644  92c787df3eae075ecd0f5b13a03dc00e1178a458  docs/plans/pipeline-measurement/001-plan.json
100644  8077968fb77bb2dbd4a74c7aea93df72d3b2b9a4  docs/plans/plan-viewer/001-plan.json
100644  d75ea8014485544a19bab8534d0a6b3c2753695b  docs/plans/planner-driven-model-routing/001-plan.json
100644  1de64b55aa7d7b0827686bdfe0ca3841457e26b8  docs/plans/planner-driven-model-routing/002-plan.json
100644  1ff779c7af008a480d2e3e61d2f64d95f6201185  docs/plans/planner-driven-model-routing/003-plan.json
100644  8155058a12643da1eccf3b27bfc0fde23adafa2a  docs/plans/review-feedback-capture/001-plan.json
100644  9875d00c93cf79baa2fd98002c67ceffd6c8a70e  docs/plans/self-drive-wake/001-plan.json
100644  8146e6240f665211a8672e4aa02a837f95c7dd54  docs/plans/skill-command-prefix/001-plan.json
100644  ef5696cde3a7b526a99aa122360ebe4ee2944797  docs/plans/workbench-ui/001-plan.json
100644  0b9e0ac3071eac4cb6c3d88151239df57c0c4907  docs/prds/argus-boundary-enforcement/effective.md
100644  ef99ed83242f2a824ebf7efc6381c8866b16a96f  docs/prds/argus-boundary-enforcement/effective.source.json
100644  2ab631c71d3c4bbe795b131357d9dc15a4620906  docs/prds/argus-ledger-lint/effective.md
100644  bc5490729aea75f13d84d7c7ad16dfb459c48868  docs/prds/argus-ledger-lint/effective.source.json
100644  ee49ef80553b7795eb4d73bcc666a87d6758f533  docs/prds/argus-unattended-drive/effective.md
100644  33994c5892308cdf0985996220249de237006169  docs/prds/argus-unattended-drive/effective.source.json
100644  1b6ed8981ac6ba6256e5cfff30e7a650e4d7d0b3  docs/prds/argus/effective.md
100644  340fc0732382a67ad23093173f0fc1206bb09b35  docs/prds/argus/effective.source.json
100644  4f610cccf5890cd024fbae945cbb680065b8af14  docs/prds/channel-inbox/effective.md
100644  e9b1171db8cba3a8cc7450756d07078d19d5ee75  docs/prds/channel-inbox/effective.source.json
100644  380e117eb32791188982a083e0f9469fb23a8eb8  docs/prds/config-driven-portability/effective.md
100644  4d0d664f8c623813bb541cecfb3fb6172dcd5a68  docs/prds/config-driven-portability/effective.source.json
100644  197efc84f2418f7353fe2fd937355899959a4ef1  docs/prds/context-pack/effective.md
100644  d12b9b968c34e44c2c2611f17e5560f4c06f99f4  docs/prds/context-pack/effective.source.json
100644  ba473feb407679543b03db72f1a07beedb60d8f9  docs/prds/contract-gate/effective.md
100644  563ad36be1f5c53761e24e60d9fdfb9284981dc4  docs/prds/contract-gate/effective.source.json
100644  b221f9d81f1d9f0c08a7d9408cc37340fd39f669  docs/prds/design-solution-shape/effective.md
100644  c50a3e4221fde05b7c20df375df81c5aa2334b29  docs/prds/design-solution-shape/effective.source.json
100644  5d383ae437ee0d6e120a5411fd7686bdf6cc84d5  docs/prds/docgate/effective.md
100644  c3a63a806d5764d28b005f033e24e8eff42ec7d5  docs/prds/docgate/effective.source.json
100644  a22a5bc20d2e6b33ab37afa6087358224eb303ec  docs/prds/e2e-verification/effective.md
100644  6b9f7ee2679fb03392f4f347ec74c00d4c8f3576  docs/prds/e2e-verification/effective.source.json
100644  67ca427582f35eda9d58e7ca2fce5fe2de96d4d3  docs/prds/ecosystem-help/effective.md
100644  9c984d7fd737c0dbd42f1fbf35ba1519f32f74c6  docs/prds/ecosystem-help/effective.source.json
100644  f1ff17bdfda897d25e7e23dca99286a58e887dd6  docs/prds/existing-repo-onboarding/effective.md
100644  c72390699c98a1919dd279a668c002bffa9e8a84  docs/prds/existing-repo-onboarding/effective.source.json
100644  9a6109bf2f4fbc8c4d07d4877e7013242d3f7b72  docs/prds/feature-brainstorm/effective.md
100644  ba81b0e4409f363feba57446d303a3ddc320cb38  docs/prds/feature-brainstorm/effective.source.json
100644  5179429d25715a363159d8d96bea317dda345214  docs/prds/feature-implementation-drive/effective.md
100644  cf97f24eda4e0b9e756a3732e2946b58c1fcb052  docs/prds/feature-implementation-drive/effective.source.json
100644  75abb8addf636e2385382009288e382347b6e81c  docs/prds/impl-review/effective.md
100644  1f8e2f6bd58ed88632ac0ae4749c381d94daf7dd  docs/prds/impl-review/effective.source.json
100644  de8808997f7f7215ef5bed3d79ff2ee204da5113  docs/prds/implement-now/effective.md
100644  3c8f1ed83dcb08dfffa6cf9b9e42fcc43525e479  docs/prds/implement-now/effective.source.json
100644  d38f0b02849ceed831e5e9ba2fc379b30c3a5de9  docs/prds/implementation-verification/effective.md
100644  ad75ea2c2f14bb365088524d8c0e6af8937fb073  docs/prds/implementation-verification/effective.source.json
100644  e96050a51e5bbafaa6bad482f6f9a6e64f14b781  docs/prds/incident-machinery/effective.md
100644  7feb48c68d41f1588902495ba0cdc39e4b439627  docs/prds/incident-machinery/effective.source.json
100644  b73696581c5dbbc5d6a14e3dd5ed28d313d26e49  docs/prds/init-hardening/effective.md
100644  43a981d3234fb3b5d15027d1caa4a58ef0670670  docs/prds/init-hardening/effective.source.json
100644  95ab0d0b3e3e42a0ac5a5ad105da28bc88345ef7  docs/prds/installer/effective.md
100644  282312e8bb38b2a60fbbb35e2aa851ea2b42f699  docs/prds/installer/effective.source.json
100644  297db24e3be1215f84da6486f86a312ba063df8f  docs/prds/local-work-tracker/effective.md
100644  99b7cf27fe9378f1281fa81d4c9ce7193d3cdfb3  docs/prds/local-work-tracker/effective.source.json
100644  f34d05d0cc198687559cb861019e06c1c296a71b  docs/prds/maintenance-sweep/effective.md
100644  b84505250c1db4173f9f756790732386fa3ba748  docs/prds/maintenance-sweep/effective.source.json
100644  8496e43b11ea95ca3681fc0b879e07584500b9d3  docs/prds/pipeline-measurement/effective.md
100644  93435c3a25f238b05c5088945aebfe758ad99e70  docs/prds/pipeline-measurement/effective.source.json
100644  1d3b4788cd15c5ba2a2150da6a8594439ae3a303  docs/prds/plan-new/effective.md
100644  c381e252c02eb2c3a2f66ba3893b3a026c7f1646  docs/prds/plan-new/effective.source.json
100644  2b982211f832421f2e41be323f48a7fc78cf24b3  docs/prds/plan-viewer/effective.md
100644  9b7f3c50e883c8ada61ab9b00c3314a7f5bef58c  docs/prds/plan-viewer/effective.source.json
100644  2d753166d34bc523627c754afb80e840441df09d  docs/prds/planner-driven-model-routing/effective.md
100644  4c3d5034b2e55cbe00939a37fa8565dce7ab6aee  docs/prds/planner-driven-model-routing/effective.source.json
100644  03962518bf28bdd107987993fd52f951efb1fa91  docs/prds/prd-review/effective.md
100644  e2d22d6fce358fe2a19ef789103e56c683be843c  docs/prds/prd-review/effective.source.json
100644  bc8a2b5d98bd8a154b2bcc7ed492bfe8060d1876  docs/prds/read-to-rule/effective.md
100644  1621e8a26834d87fbde42093d56ca4b987695c2e  docs/prds/read-to-rule/effective.source.json
100644  7881e046fa2fe1c40fc8c9c9fdf3cca683e4c281  docs/prds/review-feedback-capture/effective.md
100644  adef29d0aa44cd03cadcca75a02fb837b14f153d  docs/prds/review-feedback-capture/effective.source.json
100644  ee81f089e8b8db40116b98d6805884f82ecbbcfd  docs/prds/self-drive-wake/effective.md
100644  d7fdd9c1df080e3acafd4756569cefc09771b89f  docs/prds/self-drive-wake/effective.source.json
100644  1228f10bad3088a49d5f15a6f2087d3ad9c50b16  docs/prds/skill-command-prefix/effective.md
100644  063af95ee2a0aba2c12ecb5e82e4fa8cad8223c0  docs/prds/skill-command-prefix/effective.source.json
100644  e9b68208e9d6f37d42390574e0114863e17c0790  docs/prds/tech-design-brainstorm/effective.md
100644  c10c440d25fd72d099fdf4088e0db4e558ab8e2c  docs/prds/tech-design-brainstorm/effective.source.json
100644  ced9f3e00d6cfd7e68df261701b11605fcf54af3  docs/prds/tech-design-review/effective.md
100644  e393592419c611497a0aa7815b1525420ae9bbd3  docs/prds/tech-design-review/effective.source.json
100644  1c34bb8b1c28f70dccd72e44c940ae13e3e99679  docs/prds/workbench-ui/effective.md
100644  a403abbdde1c9815471537d5adf9bf28b6e4c434  docs/prds/workbench-ui/effective.source.json
100644  d2feee9d7cd68a6bfcf10787b9709761e3ae0c07  docs/prds/worker-transcript-metrics/effective.md
100644  e34d3ff531250f66daf348589f9bfbcb58de6780  docs/prds/worker-transcript-metrics/effective.source.json
100644  d25c770f02c1ef6bddf26ca3882a835978a34349  docs/tech-design/index.json
100644  4079efc5007fab8ffef75f3a52ddefd7d1bff8c2  docs/tech-design/index.md
```

### alias

```text
mode  git-object  source-path
120000  47dc3e3d863cfb5727b87d785d09abf9743c0a72  CLAUDE.md
```

### fixture-testdata

```text
mode  git-object  source-path
100644  e52f7c190cd55d680c72c648913eb6989760515a  .pyxis/skills/argus/fixtures/README.md
100644  bd6a638729cef9c0cd49c88cd8d2b2ca36ab35b0  .pyxis/skills/argus/fixtures/breaking/cases.md
100644  b60d037db2867affd390e152d341159e46ec51bc  .pyxis/skills/argus/fixtures/briefs/exemplars.md
100644  32ead07d53d7464a23d96c08cace40d401a8b7db  .pyxis/skills/argus/fixtures/dinv-defects/README.md
100644  3d939a4cbc435c3e71cbac11c02a3b092539e742  .pyxis/skills/argus/fixtures/eval-report.md
100644  183959b8d4a7c899ebe0155e4ea1bb490435dcad  .pyxis/skills/argus/fixtures/injection/corpus.md
100644  bbe18ff71ded0c687ddf2c718b7974c035a1a75f  .pyxis/skills/argus/fixtures/legality/cases.md
100644  f7aee7b644fd0c6f4038ef5b0329717dccbc6bad  .pyxis/skills/e2e-review/fixtures/catch-corpus/README.md
100644  81b3c3a64f38ae9ae068f2282fe4fbb95b561a60  .pyxis/skills/e2e-review/fixtures/catch-corpus/coverage-gap-1/e2e-design.md
100644  8ae0563a3450dea278284c06d60abfd661bb849e  .pyxis/skills/e2e-review/fixtures/catch-corpus/coverage-gap-1/prd.md
100644  08bf47e385708df802962ab6de14528db86599f1  .pyxis/skills/e2e-review/fixtures/catch-corpus/coverage-gap-2/design.md
100644  2ff70bbca49fceb6dd4453c192c4f638a0e5e507  .pyxis/skills/e2e-review/fixtures/catch-corpus/coverage-gap-2/e2e-design.md
100644  97311c7de8983ea65007474cc679ab3ba899810e  .pyxis/skills/e2e-review/fixtures/catch-corpus/coverage-gap-2/prd.md
100644  f351fe1393c066f9781b5e9d5aad6f1fe4c7ff28  .pyxis/skills/e2e-review/fixtures/catch-corpus/coverage-gap-clean/e2e-design.md
100644  bbefb7c739ace2ac2c3925111db37d389159885b  .pyxis/skills/e2e-review/fixtures/catch-corpus/coverage-gap-clean/prd.md
100644  d7a950adb71a3f360a0ff6b297133a33814b82ad  .pyxis/skills/e2e-review/fixtures/catch-corpus/harness-hole-1/e2e-design.md
100644  dfde755359d80bede5724c66d857d06d9e357793  .pyxis/skills/e2e-review/fixtures/catch-corpus/harness-hole-2/e2e-design.md
100644  d3acdb34d716cf315ef10f67f932f816da2d9170  .pyxis/skills/e2e-review/fixtures/catch-corpus/harness-hole-3/e2e-design.md
100644  b5b18731f377dcbd42f614c5fa93133bc83f35a8  .pyxis/skills/e2e-review/fixtures/catch-corpus/harness-hole-4/e2e-design.md
100644  a57074d79f81709f8d2bd14006c17a6c2870810a  .pyxis/skills/e2e-review/fixtures/catch-corpus/harness-hole-5/e2e-design.md
100644  d450ee9836885ae5050306be06628e18b5258b54  .pyxis/skills/e2e-review/fixtures/catch-corpus/harness-hole-6/e2e-design.md
100644  b1a85cc21295ec6cd77a32e416f85c0340184cd5  .pyxis/skills/e2e-review/fixtures/catch-corpus/harness-hole-clean/e2e-design.md
100644  e1b9739fcc38b8df44e15c17b688029d7f6a8c68  .pyxis/skills/e2e-review/fixtures/catch-corpus/hollow-scenario-1/e2e-design.md
100644  ab17ec7e6cf830b6e44ef1171c7e49f5a4a2151d  .pyxis/skills/e2e-review/fixtures/catch-corpus/hollow-scenario-2/e2e-design.md
100644  6909b21d80b5c11a3c0bedf6d89b9a6805f7209e  .pyxis/skills/e2e-review/fixtures/catch-corpus/hollow-scenario-clean/e2e-design.md
100644  9d3b96db86e7f2bac97b36c02695fc04cedc8535  .pyxis/skills/e2e-review/fixtures/catch-corpus/run-log.md
100644  807714fb843762644ab170c9e113dd2b8b87f8cd  .pyxis/skills/feature-brainstorm/fixtures/README.md
100644  43579d899d3ded8c3611370336bbd8e547060872  .pyxis/skills/feature-brainstorm/fixtures/contradiction.md
100644  8aa4918997330d9e29e537071528ec33d99c7292  .pyxis/skills/feature-brainstorm/fixtures/covered.md
100644  bbc866c0492faa2492df29fcbb82749d9b5fcaba  .pyxis/skills/feature-brainstorm/fixtures/eval-report-2026-07-09.md
100644  8f8c9c1a408b8330d7d749770108d9d438fb1cf9  .pyxis/skills/feature-brainstorm/fixtures/eval-report-2026-07-10.md
100644  702c4d3b1f70054120f670920f51770ba5bc39a1  .pyxis/skills/feature-brainstorm/fixtures/golden.md
100644  96591efee89bd02843d8131a21757090535a8246  .pyxis/skills/feature-brainstorm/fixtures/injection.md
100644  bf6cc007f48c158aff7cf84cc981aec00e638828  .pyxis/skills/feature-brainstorm/fixtures/question-form.md
100644  4715fb1aedda58b5126c056bfd4701b54773f23b  .pyxis/skills/feature-brainstorm/fixtures/records/defect-deleted-supersession.md
100644  95f3bf9581430b7912bf44049bc76d15cc7eaa08  .pyxis/skills/feature-brainstorm/fixtures/records/defect-double-gate.md
100644  a534a780e94ddca434cadbc10c15f351402751e9  .pyxis/skills/feature-brainstorm/fixtures/records/defect-unanchored-decision.md
100644  6937c037ea294baa19464e98ed2a8a75f69986dd  .pyxis/skills/feature-brainstorm/fixtures/records/defect-untyped-fog.md
100644  e0c9383a2c927e08fc7a30f31399d80ab3871a1f  .pyxis/skills/feature-brainstorm/fixtures/records/weekly-digest.md
100644  f4676711bb359c46b5a450c6ad711ecf9d5c6133  .pyxis/skills/feature-brainstorm/fixtures/reopen.md
100644  4aaba69ebaff9a151b19b973cb45f207f29da506  .pyxis/skills/feature-brainstorm/fixtures/resume.md
100644  d3378e46e9b56c9aaa01012079adcc1d02c7c28f  .pyxis/skills/feature-brainstorm/fixtures/silence.md
100644  c83b7986601b45bec1342542474cb83763d8cda9  .pyxis/skills/feature-brainstorm/fixtures/under-specified.md
100644  f431a6e2aa3876e06ed09a0c0a5ff48650b08f10  .pyxis/skills/help/fixtures/README.md
100644  9060792707bfa27f139dae86dea3a99a3932216a  .pyxis/skills/help/fixtures/eval-report-2026-07-23.md
100644  baa563aa6982dc9a1e29b4b86cf65383f96ff3ca  .pyxis/skills/help/fixtures/routing-eval.md
100644  673777db5bb37c513889051ae953a5cabfa4e290  .pyxis/skills/impl-review/fixtures/budgets.json
100644  9845d2277973445377e688654ff40a0748bdc8d8  .pyxis/skills/impl-review/fixtures/families/eval-1-spec-fidelity.json
100644  4af0ca58a98961da1af3ed296376773e2109ba0e  .pyxis/skills/impl-review/fixtures/families/eval-2-design-conformance.json
100644  e7215a3d1cdadba3dcd936a5c4cd74537354d527  .pyxis/skills/impl-review/fixtures/families/eval-3-abuse-deep-band.json
100644  25b0ecf6d1a7d04550486f542bb4370ac5bcc184  .pyxis/skills/impl-review/fixtures/families/eval-4-floor-exclusion.json
100644  46dc1222020ac317b4ccc34507bfcd0d0c019bc5  .pyxis/skills/impl-review/fixtures/families/eval-5-injection-invariance.json
100644  7358b8be77d39db36239f57b21eb481c682b8024  .pyxis/skills/impl-review/fixtures/families/eval-6-nit-discipline.json
100644  d5567416552781b436803b207bc05b74a69fd04a  .pyxis/skills/impl-review/fixtures/families/eval-7-correctness-critic.json
100644  6892a4115922ca3bc26f910acdcbdc3ec2ac5e16  .pyxis/skills/impl-review/fixtures/families/eval-8-truncation-resistance.json
100644  44e4c363535602d543d0e148f133e43a6ef5bc6c  .pyxis/skills/impl-review/fixtures/provenance.json
100644  9a8c58dc3f653ba717cdc01e7c4d5954d503b517  .pyxis/skills/prd-review/fixtures/README.md
100644  3dfc06bfcc00c3ab4bdc01af02fc48537007ee69  .pyxis/skills/prd-review/fixtures/amendment-moment.md
100644  a157b5a2fc5a416d80ecf642c865b3aea19f3f85  .pyxis/skills/prd-review/fixtures/cap-termination.md
100644  f9883e5c37473e575d8390db5baed2ceb325d933  .pyxis/skills/prd-review/fixtures/clean.md
100644  cebf8559071c097f7d4d2c61809266ab6daae1a0  .pyxis/skills/prd-review/fixtures/decline-escalate.md
100644  5e15f520025ddf1fc957c40605d535f037d41fa1  .pyxis/skills/prd-review/fixtures/eval-report-2026-07-10.md
100644  d2efb62c6094b5364bb57a378bcc726478e3682f  .pyxis/skills/prd-review/fixtures/injection.md
100644  08daf2d44111c02e2b4375a668c4010dafe6e6c9  .pyxis/skills/prd-review/fixtures/lens-defects.md
100644  1ed6dd4ad9b2022b10c2730560fd492df865382b  .pyxis/skills/prd-review/fixtures/records/defect-brief-without-escalate.md
100644  ba3d08883221e67ddf067e9b4e5d3624c9561299  .pyxis/skills/prd-review/fixtures/records/defect-header-history-mismatch.md
100644  ea3d4109e6520e44f81cf9f084ae05ffb628b483  .pyxis/skills/prd-review/fixtures/records/defect-missing-acceptance.md
100644  8f5d74e364af7be2d0ea4ea7ecd94f60d478ae19  .pyxis/skills/prd-review/fixtures/records/defect-missing-cal-citation.md
100644  a406453a56588768113266078a0159136b8a1b85  .pyxis/skills/prd-review/fixtures/records/defect-second-live-verdict.md
100644  eee0c5bd61a4f4721e8604306a07762bae0c37b4  .pyxis/skills/prd-review/fixtures/records/defect-third-round.md
100644  f4d6600e1d9c4a64c00ba7fa35adbb66c040fd3f  .pyxis/skills/prd-review/fixtures/records/queue-widget-000.md
100644  016f58483b90ee25c5299409502feac3dc44bdd4  .pyxis/skills/prd-review/fixtures/scope-lock.md
100644  b16476d8bcec260a4ff8627c2be96cabcf3b6c99  .pyxis/skills/prd-review/fixtures/subjects/queue-widget-000.md
100644  0ed5756f4541ec8971c86a5b9e4d097239a9f36c  .pyxis/skills/tech-design-brainstorm/fixtures/README.md
100644  e080c0d69e2fedb49e2e0bf7390ee2bcc735d565  .pyxis/skills/tech-design-brainstorm/fixtures/bounce.md
100644  7a5ed631ab38bc1fb60d485ae03a33997068b5f0  .pyxis/skills/tech-design-brainstorm/fixtures/contradiction.md
100644  eef58b30ccaaae709d660a2b65e00a3560cfccae  .pyxis/skills/tech-design-brainstorm/fixtures/covered.md
100644  208f59646ef9670c7c6c0754fa3d5caabdda1b30  .pyxis/skills/tech-design-brainstorm/fixtures/eval-report-2026-07-10.md
100644  080ee6c1e2289340221a86ae266858270dcf888e  .pyxis/skills/tech-design-brainstorm/fixtures/extension.md
100644  399b62ddf5fd78c0d64649141cc2e8195d8b96e1  .pyxis/skills/tech-design-brainstorm/fixtures/false-cover.md
100644  cf96c2228122c516af0658c299945b1ecaf4ca30  .pyxis/skills/tech-design-brainstorm/fixtures/glossary.md
100644  de4498d66b9cb0907e108eafe3aa330c1287d4af  .pyxis/skills/tech-design-brainstorm/fixtures/golden.md
100644  c8b13bf7b8ed4ed5851134578ee018854700ac86  .pyxis/skills/tech-design-brainstorm/fixtures/injection.md
100644  a3cdda26d8223cfd491ea2d326f7555421e4c51d  .pyxis/skills/tech-design-brainstorm/fixtures/overrides.md
100644  b198efd3e9d0143851dabd8dc7815ca077395805  .pyxis/skills/tech-design-brainstorm/fixtures/panel-quality.md
100644  5b9769fa7a49f4bae477cac4e8f1918229aa994a  .pyxis/skills/tech-design-brainstorm/fixtures/question-form.md
100644  aa8180e6b33fce7dc04b2fe7944ac89f9878901a  .pyxis/skills/tech-design-brainstorm/fixtures/records/coverage-no-boundaries.md
100644  baf3cc9f9a7da9c3f40e04426bd830dca96976c1  .pyxis/skills/tech-design-brainstorm/fixtures/records/coverage-unmapped-seams.md
100644  2cd9ecb19e8f21a0fe451042b5941520cf2d454c  .pyxis/skills/tech-design-brainstorm/fixtures/records/defect-double-gate.md
100644  0584d62b8ed1b9e20a8e84ba479af71d8e4c2b87  .pyxis/skills/tech-design-brainstorm/fixtures/records/defect-missing-panel-log.md
100644  c7c75727775473dd880ba3f5a3d7407993c0fb97  .pyxis/skills/tech-design-brainstorm/fixtures/records/defect-single-alt-fired-panel.md
100644  16074eade4165ec43319bcb5d7a66e51c0ba74b4  .pyxis/skills/tech-design-brainstorm/fixtures/records/defect-unanchored-decision.md
100644  cdcbb9dc6b6bf8859df134589873f63064ed107b  .pyxis/skills/tech-design-brainstorm/fixtures/records/defect-unscored-alt.md
100644  9366506406e34ff6743fa22bc863f6ad8375c944  .pyxis/skills/tech-design-brainstorm/fixtures/records/defect-untyped-fog.md
100644  1c7b29f874667a9c949c10177a7e9d6662192047  .pyxis/skills/tech-design-brainstorm/fixtures/records/fleet-feed-tech-design.md
100644  37a9465198d5a16987a8bfbcb0166e9475374b7f  .pyxis/skills/tech-design-brainstorm/fixtures/refusals.md
100644  2198519fea799fc3d4002a90146532ab48840b9b  .pyxis/skills/tech-design-brainstorm/fixtures/reopen.md
100644  cb325cc7f7807e6d88b146e2212c3634a2754a41  .pyxis/skills/tech-design-brainstorm/fixtures/resume.md
100644  f492a55c9ae7e407c10c690a2b33e9e7ecab3062  .pyxis/skills/tech-design-brainstorm/fixtures/silence.md
100644  fd3ecc6040205ef0f7769b51b485e7e60aece172  .pyxis/skills/tech-design-brainstorm/fixtures/under-specified.md
100644  06cada0a58be4d8a0be0d4df72254194b31547ed  .pyxis/skills/tech-design-review/fixtures/README.md
100644  995f45b5f86f4da47daf02b425de30c1dd268fb1  .pyxis/skills/tech-design-review/fixtures/clean.md
100644  baccb30809bc4af03c387b2aac2e66360d5ec2a5  .pyxis/skills/tech-design-review/fixtures/decline-escalate.md
100644  cf48e40b3c437bd596627cc77751ae44d38a6491  .pyxis/skills/tech-design-review/fixtures/eval-report-2026-07-10.md
100644  ac39823eb6675ea0f3a0c80c59bf1df6230a0f7e  .pyxis/skills/tech-design-review/fixtures/injection.md
100644  de3f572876da0c22efe179d457bd341414f4fd8b  .pyxis/skills/tech-design-review/fixtures/lens-defects.md
100644  93ea39b2248aaede70ea505776f5429ffbe01c96  .pyxis/skills/tech-design-review/fixtures/records/defect-brief-without-escalate.md
100644  264c2de345d55b9ffa1c24d38348c6eb39685be4  .pyxis/skills/tech-design-review/fixtures/records/defect-halfless-evidence.md
100644  1164e93bdc30284ec85e4a7fc2a42c1f9e38bec3  .pyxis/skills/tech-design-review/fixtures/records/defect-header-history-mismatch.md
100644  462a5d8046850678cbd0b8c74a2588ee25e689ed  .pyxis/skills/tech-design-review/fixtures/records/defect-missing-acceptance.md
100644  72b27847bf5aa3550e73f865f32011a0915d4f33  .pyxis/skills/tech-design-review/fixtures/records/defect-missing-cal-citation.md
100644  13ddc1f76d5a2703d530515ba720f1fe10ad9432  .pyxis/skills/tech-design-review/fixtures/records/defect-second-live-verdict.md
100644  2f3437950c9f8dbb35ada738de757f8f55cd230a  .pyxis/skills/tech-design-review/fixtures/records/defect-target-content-dispute.md
100644  6ba31bc60850f03db3ea37336b5ebea40849dfed  .pyxis/skills/tech-design-review/fixtures/records/defect-targetless-supersession.md
100644  cdb6e70738a6ba5d617f9afb50a30f4179da9bde  .pyxis/skills/tech-design-review/fixtures/records/defect-third-round.md
100644  27e6620536d063092d5813351d3f3cccaff50441  .pyxis/skills/tech-design-review/fixtures/records/queue-feed.md
100644  b34b072dbc4c256a6cc8e17618a1078acf6cc409  .pyxis/skills/tech-design-review/fixtures/scope-lock+cap.md
100644  07cfbcc8e0bffe51cc02c417293ed2d3273fa337  .pyxis/skills/tech-design-review/fixtures/subjects/queue-feed-shape.md
100644  aea7277ee26ab4131dca33991a6637ca8dca8883  .pyxis/skills/tech-design-review/fixtures/subjects/queue-feed.md
100644  3a431a3cd7c81cda3612ff57eb6b8efdb603031f  .pyxis/skills/tech-design-review/fixtures/supersession.md
100644  7e7a3b634b668f1c9eca7d0f6c7bfa56e5a171c2  cmd/carina/testdata/work-item-list.schema.json
100644  cb84a5d9601ab07319de5d46ec5f944387338c84  cmd/carina/testdata/work-item-search.schema.json
100644  b0272cf65ac245927b109b888dcf822f40c8f2b3  cmd/carina/testdata/work-item.schema.json
100644  599342d12a378ba5a81a4d8a96d997c6d398b9e3  internal/adopt/testdata/golden-plan.json
100644  86636830dc6d60defebbaaa146abe6a18e5040b7  internal/adopt/testdata/golden/applied-plan.md
100644  5fec96c4e077c31f68210c8d306143f27e3e36e8  internal/adopt/testdata/golden/draft-plan.md
100644  bf4fc69b4d7d1c9a21758ed846ace98ba5cea193  internal/docgate/contractgate/testdata/compil-buildon-cycle/docs/prds/demo/effective.source.json
100644  572e717568624e8c4edfb7d9598d99a20b15ff02  internal/docgate/contractgate/testdata/compil-buildon-cycle/docs/tech-design/demo-a.md
100644  cbee5438eba82dc93c07d77bbb7921061f733890  internal/docgate/contractgate/testdata/compil-buildon-cycle/docs/tech-design/demo-b.md
100644  cdd62d8f17e0741dcd15257bbfc89fc1934c1b0b  internal/docgate/contractgate/testdata/compil-dangling-supersession/docs/prds/demo/effective.source.json
100644  69c83712f4a5a85d5152fadc733a3f2fa5735afe  internal/docgate/contractgate/testdata/compil-dangling-supersession/docs/tech-design/demo-v1.md
100644  bf4fc69b4d7d1c9a21758ed846ace98ba5cea193  internal/docgate/contractgate/testdata/compil-orphan-serves/docs/prds/demo/effective.source.json
100644  1a7fa80531a638c2106ce3bdeb8f54669c7ffc90  internal/docgate/contractgate/testdata/compil-orphan-serves/docs/tech-design/demo-v1.md
100644  1c50291bf1474f9054a063c4ed358846c1dcf0a5  internal/docgate/contractgate/testdata/dangling-citation/docs/prds/demo/000-base.md
100644  e4a838d3cb444b7debb1e68fdf2627352766d2d4  internal/docgate/contractgate/testdata/dangling-citation/docs/prds/demo/effective.source.json
100644  5866a33692f63ca331cfcf01fa8b82f272a570b8  internal/docgate/contractgate/testdata/dangling-citation/docs/tech-design/demo-v1.md
100644  bed53c6e9f5c6d6e6466fc7b4f44b231d3ca98ec  internal/docgate/contractgate/testdata/golden/struct-nofit-defect-report.json
100644  39cf414ad64c4e1ba43c1b16a7be08d1fd4ba804  internal/docgate/contractgate/testdata/golden/struct-nofit-defect-report.md
100644  39ae9228eefcd3ac462c37a4671231ff139af374  internal/docgate/contractgate/testdata/malformed/docs/prds/demo/effective.source.json
100644  f5c42f2dd52227912a694b48c8deb530e101a91d  internal/docgate/contractgate/testdata/missing/docs/prds/demo/000-base.md
100644  adc3870479882e9a25bb54f9490930b3cca49567  internal/docgate/contractgate/testdata/no-designs/docs/prds/demo/effective.source.json
100644  4d59f040abe2ee00646d1707e095b2a628900792  internal/docgate/contractgate/testdata/no-designs/docs/tech-design/other-v1.md
100644  e5192ae30a00135c63cfbfbebe240e391f6c6b6f  internal/docgate/contractgate/testdata/sevmap-coverage-blocking/.pyxis/severity-map.yaml
100644  97a9e8f746245e7507b7e7c5193d98c677cfd23b  internal/docgate/contractgate/testdata/sevmap-coverage-blocking/docs/prds/demo/effective.source.json
100644  f9c9c8999a30d066d9f1d7c9dc6bdc5fa3b5da28  internal/docgate/contractgate/testdata/sevmap-coverage-blocking/docs/tech-design/demo-v1.md
100644  8a4dc6a9bb5b35843ac13a963dbcc85f621ec839  internal/docgate/contractgate/testdata/sevmap-malformed/.pyxis/severity-map.yaml
100644  86c13e87a1e9db61f02619c2bb241e6bdf0cc022  internal/docgate/contractgate/testdata/stale-serve/docs/prds/demo/effective.source.json
100644  b3f2e5af0aebbced12a46d3aff643fb4efaaaf5b  internal/docgate/contractgate/testdata/stale-serve/docs/tech-design/demo-v1.md
100644  3f7cb5bfeb5f10fa2d70526828ccc45600be5103  internal/docgate/contractgate/testdata/struct-nodod/docs/prds/demo/effective.source.json
100644  69c83712f4a5a85d5152fadc733a3f2fa5735afe  internal/docgate/contractgate/testdata/struct-nodod/docs/tech-design/demo-v1.md
100644  457023955bc364ffc00b159cda41fa3e0a1eae62  internal/docgate/contractgate/testdata/struct-nofit/docs/prds/demo/effective.source.json
100644  69c83712f4a5a85d5152fadc733a3f2fa5735afe  internal/docgate/contractgate/testdata/struct-nofit/docs/tech-design/demo-v1.md
100644  b5d432fd6bb857a494abe20af1ae8bf2030dc8b1  internal/docgate/contractgate/testdata/struct-noverify/docs/prds/demo/effective.source.json
100644  69c83712f4a5a85d5152fadc733a3f2fa5735afe  internal/docgate/contractgate/testdata/struct-noverify/docs/tech-design/demo-v1.md
100644  97a9e8f746245e7507b7e7c5193d98c677cfd23b  internal/docgate/contractgate/testdata/uncovered-must/docs/prds/demo/effective.source.json
100644  f9c9c8999a30d066d9f1d7c9dc6bdc5fa3b5da28  internal/docgate/contractgate/testdata/uncovered-must/docs/tech-design/demo-v1.md
100644  bf4fc69b4d7d1c9a21758ed846ace98ba5cea193  internal/docgate/contractgate/testdata/wellformed/docs/prds/demo/effective.source.json
100644  69c83712f4a5a85d5152fadc733a3f2fa5735afe  internal/docgate/contractgate/testdata/wellformed/docs/tech-design/demo-v1.md
100644  2607cb70e3fd42a7bf9d66d0532cd0262fe07c8b  internal/docgate/feedback/testdata/subject-alpha/001-doc-review.md
100644  fc70c9c26c62965be663ae0a564e54f4bcf85477  internal/docgate/feedback/testdata/subject-alpha/002-code-host.md
100644  9f63b460ef46149b544914f372e10644d54fe383  internal/docgate/feedback/testdata/subject-alpha/effective.source.json
100644  88b1e71255c4ca52ef31b42d93e6483ded33076a  internal/docgate/fold/testdata/DEFECTS.md
100644  83cf42a60cd0faec58101e35395a0d1ff1868ecf  internal/docgate/fold/testdata/MANIFEST.md
100644  4f3dd20b5636758a552acb380384d3386796f3c6  internal/docgate/fold/testdata/fixture-zero/000-base.md
100644  51edb49d379ff528f05e00c7e070962a1dec1684  internal/docgate/fold/testdata/ops-suite/000-base.md
100644  dd16c00b06827a801eca135e346642d923126585  internal/docgate/fold/testdata/ops-suite/001-shard-limits.md
100644  7d4cba308c87ad556f3a9614c24310c0ef6218bb  internal/docgate/fold/testdata/ops-suite/expect/effective.md
100644  e7748c72d3c4f6a6e403cd27c784abe1a8c683f3  internal/docgate/fold/testdata/ops-suite/expect/effective.source.json
100644  e6183f96abba4833558ac339e26ef9a99486f647  internal/docgate/gate/testdata/argus-ledger-lint/MANIFEST.md
100644  d5ec7ac6b11fc46d5dcb1c5c9a3648800f3456f2  internal/e2e/design/testdata/fixtures/approval-engine-defective/assertion-sighted.md
100644  49598c0d6be9440ff7a055b6c8e18171b3a81727  internal/e2e/design/testdata/fixtures/approval-engine-defective/cooperative-only.md
100644  528b8688d7bf67f43bd09efa2610f9d617980230  internal/e2e/design/testdata/fixtures/approval-engine-defective/improvising-unrecorded.md
100644  ac85975c25520dde4832b600fa7f4403bdd0a878  internal/e2e/design/testdata/fixtures/approval-engine/design-output.md
100644  998957844448e494d9cfb02fbda40b9c8b9aca72  internal/e2e/design/testdata/fixtures/approval-engine/design.md
100644  cbec8069fed516eef5bb9a4929bfd5a87e3a828c  internal/e2e/design/testdata/fixtures/approval-engine/prd.md
100644  d3c5467060bcba73283ccd1919b2c298c44495bd  internal/e2e/design/testdata/fixtures/unsettleable-harness/design-output.md
100644  758b5101382e64515b0c48d9b9a7ebe6de6fb54b  internal/e2e/design/testdata/fixtures/unsettleable-harness/design.md
100644  a551a7b44f058f0d71d31c1bcf2445d206d1907f  internal/e2e/design/testdata/fixtures/unsettleable-harness/prd.md
100644  653a53bf12bc43838328ace5874c547836a5c781  internal/e2e/design/testdata/fixtures/webhook-relay/design-output.md
100644  786822822578e7a872987defe9952622abbfd542  internal/e2e/design/testdata/fixtures/webhook-relay/design.md
100644  ba8362f52585ac00722e491c1a36c15e4eafefcb  internal/e2e/design/testdata/fixtures/webhook-relay/prd.md
100644  b8145cf7323216069e4f5545eeffc7cedb2ec22a  internal/e2e/docclass/testdata/defect-artifact-duplicate.md
100644  24ccbb9d743f9e29820df9542cd39ec9d467dd33  internal/e2e/docclass/testdata/defect-basis-grammar.md
100644  57f146489c11e93a773c50d2877fac464fc58fd1  internal/e2e/docclass/testdata/defect-blocking-open-item-approved.md
100644  102394add78ce7c1acd91d7087cb61826fc0a0bf  internal/e2e/docclass/testdata/defect-build-fact-unknown-scenario.md
100644  68f1908623ecd8f5ee66c92f452ebb7954551d0f  internal/e2e/docclass/testdata/defect-build-facts-count.md
100644  3c6888d12cd9bfb044068d8ff16961c41bc46327  internal/e2e/docclass/testdata/defect-cadence-enum.md
100644  034e0b40571dc354b3bf9ee1d1bbb7be01346940  internal/e2e/docclass/testdata/defect-cost-class-enum.md
100644  464e61c7cc923658544736073c4e8f82725b8233  internal/e2e/docclass/testdata/defect-coverage-routed.md
100644  1129a723adce4faaddc02b74aa162f1be6b0b54b  internal/e2e/docclass/testdata/defect-coverage-unknown-scenario.md
100644  cb8450623fc3410172a04601576fce00de257c32  internal/e2e/docclass/testdata/defect-harness-form-both.md
100644  a8f248a0f8b37fd5183cb28bcbcd92af3aec6c50  internal/e2e/docclass/testdata/defect-harness-slot-bare-token.md
100644  5abce30400998f9a90cc4b8a1630a3ebffe92bea  internal/e2e/docclass/testdata/defect-harness-slot-empty.md
100644  d966476919720658ef11e92d469a8d4837cc5ae7  internal/e2e/docclass/testdata/defect-harness-slot-missing.md
100644  d3f43a8d62173a208f9fe0bba65d0290603c359e  internal/e2e/docclass/testdata/defect-harness-slot-unknown.md
100644  aea02079197c9e5689cd5d8e43d2ed523632b72b  internal/e2e/docclass/testdata/defect-header-field-missing.md
100644  75057984825a415ee8e68b2d73bb15e0cb8c60dc  internal/e2e/docclass/testdata/defect-header-field-unknown.md
100644  309be0733194008651608b43e563374a61cc2ffc  internal/e2e/docclass/testdata/defect-header-repeated.md
100644  bc8a8832074f2a9e66ecd44f3396685489d7a24e  internal/e2e/docclass/testdata/defect-manual-owner.md
100644  ccd4c3ccfa8b04616806645348f6c0ca9fb512f8  internal/e2e/docclass/testdata/defect-owned-artifact.md
100644  423566936ea7e77d123cb9b8997ca5d420756418  internal/e2e/docclass/testdata/defect-route-count-duplicate.md
100644  1aed09c70201d8bb3fe912b059ad0863481828d5  internal/e2e/docclass/testdata/defect-route-count-two.md
100644  af2694246e1543d0505fd29f3df8a9a684acfb7c  internal/e2e/docclass/testdata/defect-route-enum.md
100644  4ec9556a213e3f84287596e39d0b7bbaeb1f5a24  internal/e2e/docclass/testdata/defect-route-unknown-scenario.md
100644  1301a4894e0757c52550f35dc249d79f1b5a7e72  internal/e2e/docclass/testdata/defect-scenario-routed.md
100644  b5fb092d07e20eca36c62230fb47dc7c6923f3c4  internal/e2e/docclass/testdata/defect-section-empty.md
100644  3889f64f52bf9b0f4a410c0452eb0e46d844ea10  internal/e2e/docclass/testdata/defect-section-present.md
100644  7fe992a81d351056e58fa3dad9721909815e49a3  internal/e2e/docclass/testdata/defect-section-repeated.md
100644  3b1d527b286c6063d8a33ef0a0f2eec222424597  internal/e2e/docclass/testdata/defect-section-unknown.md
100644  34a7388593dd37ea46b432654dbca54a419a2239  internal/e2e/docclass/testdata/defect-status-enum.md
100644  a102c3f8b823ccbe36eca8118153762759c48655  internal/e2e/docclass/testdata/defect-tables-mismatch.md
100644  0bb94612734ea179c6f0392d1f08a37e61795d2c  internal/e2e/docclass/testdata/defect-template-version.md
100644  00947f0962316f2e59ca0d9d8ebd36854589d4ce  internal/e2e/docclass/testdata/golden-blocking-open-item-draft.md
100644  caf0a73655dd16cd499c454248567512e4b3aa93  internal/e2e/docclass/testdata/golden-v2-minimal.md
100644  d3480cc1e109efe947b4931ad201432a76b62673  internal/e2e/docclass/testdata/golden-v2.md
100644  bd7e23d7022d8fa8698f17ca09c329a08b8b2f80  internal/e2e/docclass/testdata/golden.md
100644  001668be285aea148b7db03ecdbcb654b027a568  internal/e2e/plan/testdata/approved.md
100644  eb5ddcdbad4f27b9bfc7e67a124683f022d0c0e4  internal/e2e/plan/testdata/draft.md
100644  15ac911e1e96402adc42863c4b5bc9b6bb810cf0  internal/e2e/report/testdata/buildability-finding.md
100644  ede9bf818a3c7419ab0bb64461cf6c86c07b544c  internal/e2e/report/testdata/golden-valid.md
100644  b6db68f8776749b27c50bdae321aa0dc291d0412  internal/executiondemand/conformancetest/testdata/golden_capsule_plan_v1_compat.json
100644  768b9529ea9cbcd83375b1a5c2c43fa535c48f8e  internal/executiondemand/conformancetest/testdata/golden_capsule_plan_v2.json
100644  a06598b33fb50473152cee42fec5ef435e760f08  internal/executiondemand/testdata/golden_plan_v1_compat.json
100644  a80850f9b50a1815755346d8a28b821848198552  internal/executiondemand/testdata/golden_plan_v1_translation.json
100644  c4f2c31d9adcb27f47d8da006e78fc4a5c4d674a  internal/executiondemand/testdata/golden_plan_v2.json
100644  0eca898fe4a98bdb2fcf4b0be737331229dfc85b  internal/executiondemand/testdata/plan_v1_source_manifest.json
100644  fb8915208c4bae902261fed95a982aa8e7bb2f76  internal/executor/e2ebasis/testdata/design.md
100644  bfc2a9f4c3ab917fa7047c9887e0e3dd35be508f  internal/executor/e2ebasis/testdata/effective-no-requirements.md
100644  9fd753cd0a29ffb7378d43dc7402cf54788320de  internal/executor/e2ebasis/testdata/effective.md
100644  166cc855609b72c6b3b296cbc7f4e99f9d6de77f  internal/executor/e2ebasis/testdata/harness.golden.json
100644  36f9ededf8ffa84a8cd34d3b145f53b291d904d0  internal/executor/e2ebasis/testdata/plan.md
100644  48c888dc5c5ee1e168b5053e45eeed480fae2d4c  internal/executor/e2ebasis/testdata/s1.golden.json
100644  cce1b6d1fde9548799e042980964fd1aa6e31f55  internal/executor/e2ebasis/testdata/s2.golden.json
100644  a2f2a3f114500ccdfff56cf7527812dc9be098a3  internal/executor/e2eorigin/testdata/design.md
100644  658e1f5fceb97e3c373ecc5526d6f76b4b030fab  internal/executor/e2eorigin/testdata/plan-harness.md
100644  ed8a41d13a9e324c8b2cb8e0d4585ab21a4f8953  internal/executor/e2eorigin/testdata/plan-mismatch.md
100644  cdfc6ac42d8183b603ee897f71e0bb4ddecaacaf  internal/executor/e2eorigin/testdata/plan-no-harness.md
100644  de3a06520e757fbc36765bba87d434f12519defd  internal/executor/runtime/cursor/testdata/native-routing-decode-fixtures.json
100644  751d4a08ceb296cbda9e85ec98851a8e77c68a0e  internal/executor/runtime/opencode/testdata/role-routing-results-v1.json
100644  4c0114e83f9036832542184babe38fe949c247ef  internal/executor/runtime/opencode/testdata/role-routing-v1.json
100644  9a51ee2bb5867e8c969932374901c60d2178ae00  internal/implreview/report/testdata/golden-valid.json
100644  997b694017b24d7acf082bb9ff65c611749d3b4d  internal/plangate/plan/testdata/golden-001-plan.json
100644  457023955bc364ffc00b159cda41fa3e0a1eae62  internal/plangate/plan/testdata/precheck-bouncing/docs/prds/demo/effective.source.json
100644  b2ce6f53e6638e8579a8ccf12d4a82f26b65e85f  internal/plangate/plan/testdata/precheck-bouncing/docs/tech-design/demo-v1.md
100644  bf4fc69b4d7d1c9a21758ed846ace98ba5cea193  internal/plangate/plan/testdata/precheck-clean/docs/prds/demo/effective.source.json
100644  b705e4593862f6b41fac36bed559db6e3bfa62ac  internal/plangate/plan/testdata/precheck-clean/docs/tech-design/demo-v1.md
100644  d2efcba963a5b2d7f9e42a2f2cbccf4eb16337f7  internal/relmanifest/testdata/golden-manifest.json
100644  0d4ab7ef082a7efebb291a2b93c1f3213cfb58c3  internal/relmanifest/testdata/manifest-unknown-key.json
100644  9ae6703d3f8447d807e0dd7e1fd2f6e50206d750  internal/relmanifest/testdata/manifest-unknown-nested-key.json
100644  3f4432c2ba7633df3316c57d2e0c7b45f93dba4c  internal/relmanifest/testdata/payload-real-shaped/.github/workflows/docs-gate.yml
100644  9baedf019d356c2f050afd404a26d370bd422096  internal/relmanifest/testdata/payload-real-shaped/.pyxis/config.yaml
100644  067faaabcd9130f5c42af2ba4278955c60886fb4  internal/relmanifest/testdata/payload-real-shaped/.pyxis/stamp/origin.yaml
100644  cfb2adddf9242adada6bfe6f5980ca7115401ed5  internal/relmanifest/testdata/payload-real-shaped/.pyxis/templates/prd-template.md
100644  5487d142ae55cd29ae052d7c1fa66e87524f54f0  internal/relmanifest/testdata/payload-real-shaped/docs/tech-design/index.json
100644  51c19e09552f3dce63cb08f3d7c321f0228c0022  internal/relmanifest/testdata/payload-real-shaped/docs/tech-design/index.md
100644  9baedf019d356c2f050afd404a26d370bd422096  internal/relmanifest/testdata/payload-unclassed/.pyxis/config.yaml
100644  cfb2adddf9242adada6bfe6f5980ca7115401ed5  internal/relmanifest/testdata/payload-unclassed/.pyxis/templates/prd-template.md
100644  571fb1fc8a3bb1821411ada0c220003c4ff1a928  internal/relmanifest/testdata/payload-unclassed/README.md
100644  0b1a02f81e1828860af33626b10240d29b9cbae1  internal/relmanifest/testdata/policy-unknown-class.yaml
100644  4edadbafcb2e8f60309923da404c09b3fa47b2a1  internal/relmanifest/testdata/policy-unknown-key.yaml
100644  9bc9ff72e4805f34c7576e078f418650c4cbebcc  internal/relmanifest/testdata/policy.yaml
100644  95f3c01c879c9f07f52e97b7cfb99759fd6483c3  internal/routingevidence/testdata/event_family_golden.json
100644  498260fe0cda7b6e68470d85b411913ca4d1e3bd  internal/verify/testdata/dryrun-parity/local-verdict.json
100644  93e8ad48adf122aa9a77a56aaa2fc698c98ebef8  internal/verify/testdata/fixture-go/.pyxis/codegate.json
100644  7a7357090ac53141d63e594353748227fbfaa9ab  internal/verify/testdata/fixture-node/.pyxis/codegate.json
100644  f7a7cb3d3085c83e9327744cd2ba02aba4c0e85b  internal/verify/testdata/fixture-node/node_modules/leftpad/package.json
100644  7e87b269ed22c2c4eee5826ebf5cf8de507de41a  internal/verify/testdata/fixture-python/.pyxis/codegate.json
```

### mutable-operational-executable-configuration

```text
mode  git-object  source-path
100644  7a3718b105910fda5aa1e973abd2e0ab4e06800c  .github/workflows/codegate-catch-suite.yml
100644  bc71b29988131a0244ce1e1ac6a91cb5d92fb0ac  .github/workflows/codegate-dry-run.yml
100644  e6d9d0bb5c91a03715e87e8cf1d56ac7e7f1d1ec  .github/workflows/codegate-release.yml
100644  b538887ab2dce62e0d229bb1090e04500066c7ec  .github/workflows/docgate-release.yml
100644  920ea1beb783453e5ea991905891b04bac79c74a  .github/workflows/docs-gate.yml
100644  f15328f0b674a8da6e4672e04cb02d3f3a4ea182  .golangci.yml
100644  adc14ac7d1c77aa5389c9c9cceb3ef1624e0a39f  .pyxis/vendor/lavish-axi/package-lock.json
100644  c7736df0c7a58182a5fd227bb4bc30093499a56d  .pyxis/vendor/lavish-axi/package.json
100644  94d9f6496bde106bd8c0ccbd876a346b9c0e20ed  .pyxis/vendor/litellm-prices/model_prices_and_context_window.json
100644  9281ada273b2a491118a4852df4f499c45a20bff  .pyxis/vendor/mermaid/package.json
100644  e6a4c232497b7ce41adcdfc3d5ff715cf9afb450  .workitems/items/1.md
100644  05382c5aabaee05b6ae821ffacec297e098ebf6e  .workitems/items/10.md
100644  6761203dfaca0945f3d6ec333220aec6149b1fa2  .workitems/items/11.md
100644  4a23328eab2befa6120ca5780ab7d8ececcc8a72  .workitems/items/12.md
100644  c1a985bfbb32557a7a1880c23408e3d95af2b26b  .workitems/items/2.md
100644  5172289845be19cd9dc5882d4ac342db0e58ccf3  .workitems/items/3.md
100644  90b39eddc6119a9806235d9298d20f7afe51171c  .workitems/items/4.md
100644  341d37cbb202a3004d760ea969f17a8febbd367c  .workitems/items/5.md
100644  6e61095c9376d05a6742f8b64afc1c165ad6c2da  .workitems/items/6.md
100644  4acfdd640e319dd5e7a81665a9665c55cbed7027  .workitems/items/7.md
100644  96ebf4b71034881f3de366939e7db9b2f12e577b  .workitems/items/8.md
100644  e7b70c3507a503a326d7b36922cc6bf79e0c2ac3  .workitems/items/9.md
100644  4bd32bbc91a7df1c896965cb425811df0a9f3a43  .workitems/signals.md
100644  8d4594c1343dbed6145655464a0c5d1b95d3ef37  internal/preflight/toolchain_floors.json
```
