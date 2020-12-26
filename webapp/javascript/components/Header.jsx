import React from 'react';
import { connect } from "react-redux";
import "react-dom";

import Spinner from "react-svg-spinner";

import DateRangePicker from "./DateRangePicker";
import DownloadButton from './DownloadButton';
import RefreshButton from './RefreshButton';
import Label from "./Label";
import NameSelector from "./NameSelector";

import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faGithub, faSlack } from '@fortawesome/free-brands-svg-icons';

import classNames from "classnames";

import { fetchNames } from "../redux/actions";

class Header extends React.Component {
  constructor(props) {
    super(props);
  }



  render() {
    return <div className="navbar">
      <h1 className="logo"></h1>
      <div className={
        classNames("labels", { visible: !this.props.areNamesLoading })
      }>
        <NameSelector/>
        {this.props.labels.filter(x => x.name !== "__name__").map(function(label) {
          return <Label key={label.name} label={label}></Label>;
        })}
      </div>
      {/* <div className={
        classNames("navbar-spinner-container", {
          visible: this.props.areNamesLoading
        })
      }>
        <Spinner color="rgba(255,255,255,0.6)" size="20px"/>
      </div> */}
      {/* <LabelsFilter /> */}
      <div className="navbar-space-filler"></div>
      <div>
        <span><a target="_blank" href="https://pyroscope.io/docs">Docs</a></span>
        &nbsp;
        &nbsp;
        <span><svg aria-hidden="true" focusable="false" data-prefix="fab" data-icon="slack" className="svg-inline--fa fa-slack fa-w-14 " role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 17 17" fill="none"><script xmlns=""/><path d="M6.233 0a1.7 1.7 0 000 3.4h1.7V1.7a1.7 1.7 0 00-1.7-1.7zm0 4.533H1.7a1.7 1.7 0 000 3.4h4.534a1.7 1.7 0 100-3.4" fill="#36C5F0"/><path d="M17 6.233a1.7 1.7 0 10-3.4 0v1.7h1.7a1.7 1.7 0 001.7-1.7zm-4.533.001V1.7a1.7 1.7 0 10-3.4 0v4.533a1.7 1.7 0 003.4 0" fill="#2EB67D"/><path d="M10.766 17a1.7 1.7 0 100-3.4h-1.7v1.7a1.7 1.7 0 001.7 1.7zm0-4.533H15.3a1.7 1.7 0 100-3.4h-4.534a1.7 1.7 0 100 3.4z" fill="#ECB22E"/><path d="M0 10.766a1.7 1.7 0 103.4 0v-1.7H1.7a1.7 1.7 0 00-1.7 1.7zm4.533 0V15.3a1.7 1.7 0 103.4 0v-4.534a1.7 1.7 0 10-3.4 0z" fill="#E01E5A"/></svg>&nbsp;<a target="_blank" href="https://pyroscope.io/slack">Slack</a></span>
        &nbsp;
        &nbsp;
        {/* <span><FontAwesomeIcon icon={faSlack} /> Join our <a target="_blank" href="https://github.com/">Slack</a></span> */}
        <span><FontAwesomeIcon icon={faGithub} />&nbsp;<a target="_blank" href="https://github.com/pyroscope-io/pyroscope">GitHub</a></span>
      </div>
      <div className={
        classNames("navbar-spinner-container", {
          visible: this.props.isJSONLoading
        })
      }>
        <Spinner color="rgba(255,255,255,0.6)" size="20px"/>
      </div>
      &nbsp;
      <RefreshButton/>
      {/* &nbsp; */}
      {/* <ZoomOutButton/> */}
      &nbsp;
      <DateRangePicker />
    </div>
  }
}

export default connect(
  (x) => x,
  { fetchNames }
)(Header);
