module LoginTests exposing (all)

import Concourse
import Dict
import Expect
import Html.Attributes exposing (href)
import Html.Events exposing (onClick)
import Login.Login as Login
import Login.Views as Views
import Message.Message as Message
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, id, style, tag, text)
import UserState exposing (UserState(..))


hasStyle : String -> String -> Views.View msg -> Expect.Expectation
hasStyle property expected view =
    Views.getStyle property view
        |> (\actual ->
                if actual == expected then
                    Expect.pass

                else
                    Expect.fail <|
                        "expected style '"
                            ++ property
                            ++ "' to be '"
                            ++ expected
                            ++ "', but was '"
                            ++ actual
                            ++ "'"
           )


all : Test
all =
    describe "login menu"
        [ test "is never more than 20% of the screen width" <|
            \_ ->
                Login.myView
                    (UserStateLoggedIn sampleUser)
                    { isUserMenuExpanded = False }
                    False
                    |> Views.getStyle "max-width"
                    |> Expect.equal "20%"
        , test "when logged out, has the login container styles" <|
            \_ ->
                Login.myView
                    UserStateLoggedOut
                    { isUserMenuExpanded = False }
                    False
                    |> Views.find "login-container"
                    |> Expect.all
                        [ hasStyle "position" "relative"
                        , hasStyle "display" "flex"
                        , hasStyle "flex-direction" "column"
                        , hasStyle "border-left" ("1px solid " ++ borderGrey)
                        , hasStyle "line-height" lineHeight
                        ]
        , test "when logged out, has a link to login" <|
            \_ ->
                Login.myView
                    UserStateLoggedOut
                    { isUserMenuExpanded = False }
                    False
                    |> Views.find "login-item"
                    |> Views.childAt 0
                    |> Expect.equal
                        (Views.a
                            Views.Unidentified
                            []
                            [ href "/sky/login" ]
                            [ Views.text "login" ]
                        )
        , test "when logged out, has the login username styles" <|
            \_ ->
                Login.myView
                    UserStateLoggedOut
                    { isUserMenuExpanded = False }
                    False
                    |> Views.find "login-item"
                    |> Expect.all
                        [ hasStyle "padding" "0 30px"
                        , hasStyle "cursor" "pointer"
                        , hasStyle "display" "flex"
                        , hasStyle "align-items" "center"
                        , hasStyle "justify-content" "center"
                        , hasStyle "flex-grow" "1"
                        ]
        , test "when pipeline is paused, draws almost-white line to the left of login container" <|
            \_ ->
                Login.myView
                    (UserStateLoggedIn sampleUser)
                    { isUserMenuExpanded = False }
                    True
                    |> Views.find "login-container"
                    |> hasStyle "border-left" ("1px solid " ++ almostWhite)
        , test "when logged in, draws lighter grey line to the left of login container" <|
            \_ ->
                Login.myView
                    (UserStateLoggedIn sampleUser)
                    { isUserMenuExpanded = False }
                    False
                    |> Views.find "login-container"
                    |> hasStyle "border-left" ("1px solid " ++ borderGrey)
        , test "when logged in, renders login container tall enough" <|
            \_ ->
                Login.myView
                    (UserStateLoggedIn sampleUser)
                    { isUserMenuExpanded = False }
                    False
                    |> Views.find "login-container"
                    |> hasStyle "line-height" lineHeight
        , test "when logged in, has the login username styles" <|
            \_ ->
                Login.myView
                    (UserStateLoggedIn sampleUser)
                    { isUserMenuExpanded = False }
                    False
                    |> Views.find "user-id"
                    |> Expect.all
                        [ hasStyle "padding" "0 30px"
                        , hasStyle "cursor" "pointer"
                        , hasStyle "display" "flex"
                        , hasStyle "align-items" "center"
                        , hasStyle "justify-content" "center"
                        , hasStyle "flex-grow" "1"
                        , Views.childAt 0
                            >> Expect.all
                                [ hasStyle "overflow" "hidden"
                                , hasStyle "text-overflow" "ellipsis"
                                ]
                        ]
        , test "shows the logged in username when the user is logged in" <|
            \_ ->
                Login.myView
                    (UserStateLoggedIn sampleUser)
                    { isUserMenuExpanded = False }
                    False
                    |> Views.find "user-id"
                    |> Views.childAt 0
                    |> Views.childAt 0
                    -- TODO maybe some kind of `hasDescendant` helper
                    |> Expect.equal (Views.text "test")
        , test "when logout is clicked, a LogOut TopLevelMessage is sent" <|
            \_ ->
                Login.myView
                    (UserStateLoggedIn sampleUser)
                    { isUserMenuExpanded = True }
                    False
                    |> Views.find "logout-button"
                    |> Views.hasAttribute (onClick Message.LogOut)
                    |> Expect.true "wrong click handler"
        , test "when logged in, shows user menu when ToggleUserMenu msg is received" <|
            \_ ->
                ( { isUserMenuExpanded = False }, [] )
                    |> Login.update Message.ToggleUserMenu
                    |> Tuple.first
                    |> (\m -> Login.myView (UserStateLoggedIn sampleUser) m False)
                    |> Views.find "logout-button"
                    |> Expect.notEqual (Views.text "")
        , test "renders user menu content when ToggleUserMenu msg is received and logged in" <|
            \_ ->
                ( { isUserMenuExpanded = False }, [] )
                    |> Login.update Message.ToggleUserMenu
                    |> Tuple.first
                    |> (\m -> Login.myView (UserStateLoggedIn sampleUser) m False)
                    |> Views.find "logout-button"
                    |> Expect.all
                        [ Views.childAt 0
                            >> Expect.equal (Views.text "logout")
                        , hasStyle "position" "absolute"
                        , hasStyle "top" "55px"
                        , hasStyle "background-color" backgroundGrey
                        , hasStyle "height" topBarHeight
                        , hasStyle "width" "100%"
                        , hasStyle "border-top" <| "1px solid " ++ borderGrey
                        , hasStyle "cursor" "pointer"
                        , hasStyle "display" "flex"
                        , hasStyle "align-items" "center"
                        , hasStyle "justify-content" "center"
                        , hasStyle "flex-grow" "1"
                        ]
        , test "when logged in, does not render the logout button" <|
            \_ ->
                Login.myView
                    (UserStateLoggedIn sampleUser)
                    { isUserMenuExpanded = False }
                    False
                    |> Views.find "logout-button"
                    |> Expect.equal (Views.text "")
        ]



-- TODO put this in a test data module


sampleUser : Concourse.User
sampleUser =
    { id = "1", userName = "test", name = "Bob", email = "bob@bob.com", teams = Dict.empty }


lineHeight : String
lineHeight =
    "54px"


topBarHeight : String
topBarHeight =
    "54px"


borderGrey : String
borderGrey =
    "#3d3c3c"


almostWhite : String
almostWhite =
    "rgba(255, 255, 255, 0.5)"


backgroundGrey =
    "#1e1d1d"
