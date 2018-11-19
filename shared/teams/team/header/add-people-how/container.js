// @flow
import * as React from 'react'
import * as ConfigGen from '../../../../actions/config-gen'
import {connect} from '../../../../util/container'
import {AddPeopleHow} from '.'
import {navigateTo, switchTo} from '../../../../actions/route-tree'
import {teamsTab} from '../../../../constants/tabs'

type OwnProps = {
  attachTo: () => ?React.Component<any>,
  onHidden: () => void,
  teamname: string,
  visible: boolean,
}

const mapDispatchToProps = (dispatch, {teamname}: OwnProps) => {
  return {
    onAddPeople: () => {
      dispatch(
        navigateTo(
          [{selected: 'team', props: {teamname}}, {selected: 'addPeople', props: {teamname}}],
          [teamsTab]
        )
      )
      dispatch(switchTo([teamsTab]))
    },
    onInvite: () => {
      dispatch(
        ConfigGen.createRequestContactPermissions({
          actionOnComplete: [
            navigateTo(
              [{selected: 'team', props: {teamname}}, {selected: 'inviteByEmail', props: {teamname}}],
              [teamsTab]
            ),
            switchTo([teamsTab]),
          ],
        })
      )
    },
  }
}

export default connect<OwnProps, _, _, _, _>(
  () => ({}),
  mapDispatchToProps,
  (s, d, o) => ({...o, ...s, ...d})
)(AddPeopleHow)
